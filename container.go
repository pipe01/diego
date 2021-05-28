package diego

import (
	"errors"
	"fmt"
	"reflect"
)

var emptyValue = reflect.Value{}

type serviceRegistration struct {
	factory  reflect.Value
	lifetime Lifetime
	instance interface{}
}

type Container struct {
	services map[reflect.Type]*serviceRegistration
}

func NewContainer() *Container {
	return &Container{
		services: map[reflect.Type]*serviceRegistration{},
	}
}

func (c *Container) Register(i interface{}, lifetime Lifetime) {
	t := reflect.TypeOf(i)

	if t.Kind() != reflect.Func {
		c.registerInstance(i, lifetime)
		return
	}

	if t.NumOut() != 1 && t.NumOut() != 2 {
		panic("Factory method must have either 1 or 2 outputs")
	}

	if t.NumOut() == 2 && !t.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		panic("Factory method's second output must be an error")
	}

	svcType := t.Out(0)

	if _, ok := c.services[svcType]; ok {
		panic(fmt.Sprintf("Type %s already registered", svcType.Name()))
	}

	c.services[svcType] = &serviceRegistration{
		factory:  reflect.ValueOf(i),
		lifetime: lifetime,
	}
}

func (c *Container) registerInstance(inst interface{}, lifetime Lifetime) {
	svcType := reflect.TypeOf(inst)

	if _, ok := c.services[svcType]; ok {
		panic(fmt.Sprintf("Type %s already registered", svcType.Name()))
	}

	c.services[svcType] = &serviceRegistration{
		instance: inst,
		lifetime: lifetime,
	}
}

func (c *Container) GetInstance(tp interface{}) (interface{}, error) {
	if tp == nil {
		panic("nil value passed to GetInstance. Make sure you pass a pointer, not an interface value")
	}

	pointer := reflect.TypeOf(tp)
	pointerVal := reflect.ValueOf(tp)

	if pointer.Kind() != reflect.Ptr {
		panic("Type parameter must be a pointer")
	}

	val, err := c.getInstance(pointer.Elem())
	if err != emptyValue && !err.IsNil() {
		return nil, err.Interface().(error)
	}

	if pointerVal.Elem() != reflect.ValueOf(nil) {
		pointerVal.Elem().Set(val)
	}

	return val.Interface(), nil
}

func (c *Container) MustGetInstance(tp interface{}) interface{} {
	v, err := c.GetInstance(tp)
	if err != nil {
		panic(err)
	}
	return v
}

func (c *Container) getInstance(svcType reflect.Type) (svc reflect.Value, err reflect.Value) {
	// If the type is directly registered, call its factory
	if reg, ok := c.services[svcType]; ok {
		svc, err = c.instantiate(reg)
	} else if svcType.Kind() == reflect.Interface {
		// Otherwise, search for a type that implements the interface

		for t, reg := range c.services {
			if t.Implements(svcType) {
				svc, err = c.instantiate(reg)
			}
		}
	} else if svcType.Kind() == reflect.Func {
		if svcType.NumIn() != 0 {
			return emptyValue, reflect.ValueOf(errors.New("lazy function parameter must have no inputs"))
		}
		if svcType.NumOut() != 1 {
			return emptyValue, reflect.ValueOf(errors.New("lazy function parameter must have a single output"))
		}

		svcType = svcType.Out(0)

		funcType := reflect.FuncOf([]reflect.Type{}, []reflect.Type{svcType}, false)
		fnc := reflect.MakeFunc(funcType, func([]reflect.Value) (results []reflect.Value) {
			svc, err := c.getInstance(svcType)
			if err != emptyValue && !err.IsNil() {
				panic(err)
			}
			return []reflect.Value{svc}
		})

		return fnc, emptyValue
	}

	if err != emptyValue && !err.IsNil() {
		return emptyValue, reflect.ValueOf(fmt.Errorf("create implementation for %s: %w", svcType.Name(), err.Interface()))
	}

	if svc != emptyValue {
		return svc, emptyValue
	}

	return emptyValue, reflect.ValueOf(&ErrorNotRegistered{svcType.Name()})
}

func (c *Container) instantiate(reg *serviceRegistration) (reflect.Value, reflect.Value) {
	if reg.instance != nil {
		return reflect.ValueOf(reg.instance), emptyValue
	}

	factoryType := reg.factory.Type()

	args := make([]reflect.Value, factoryType.NumIn())
	for i := 0; i < len(args); i++ {
		argType := factoryType.In(i)

		val, err := c.getInstance(argType)
		if err != emptyValue {
			return emptyValue, err
		}

		args[i] = val
	}

	out := reg.factory.Call(args)

	var err reflect.Value
	if len(out) > 1 {
		err = out[1]
	}

	if reg.lifetime == Singleton && (err == emptyValue || err.IsNil()) {
		reg.instance = out[0].Interface()
	}

	return out[0], err
}
