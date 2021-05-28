package diego

import (
	"errors"
	"fmt"
	"reflect"
)

var emptyValue = reflect.Value{}

var (
	ErrServiceNotRegistered = errors.New("service not registered")
)

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

func (c *Container) Register(factory interface{}, lifetime Lifetime) {
	t := reflect.TypeOf(factory)

	if t.Kind() != reflect.Func {
		panic("Invalid factory passed")
	}

	if t.NumOut() != 1 && t.NumOut() != 2 {
		panic("Factory method must have either 1 or 2 outputs")
	}

	if t.NumOut() == 2 && !t.Out(1).Implements(reflect.TypeOf(ErrServiceNotRegistered)) {
		panic("Factory method's second output must be an error")
	}

	svcType := t.Out(0)

	if _, ok := c.services[svcType]; ok {
		panic(fmt.Sprintf("Type %s already registered", svcType.Name()))
	}

	c.services[svcType] = &serviceRegistration{
		factory:  reflect.ValueOf(factory),
		lifetime: lifetime,
	}
}

func (c *Container) GetInstance(tp interface{}) (interface{}, error) {
	val, err := c.getInstance(tp)
	if err != emptyValue {
		return nil, err.Interface().(error)
	}
	return val.Interface(), nil
}

func (c *Container) getInstance(tp interface{}) (reflect.Value, reflect.Value) {
	pointer := reflect.TypeOf(tp)
	pointerVal := reflect.ValueOf(tp)

	if pointer.Kind() != reflect.Ptr {
		panic("Type parameter must be a pointer")
	}

	svcType := pointer.Elem()

	var svc, err reflect.Value

	// If the type is directly registered, call its factory
	if reg, ok := c.services[svcType]; ok {
		svc, err = c.callFactory(reg)
	} else if svcType.Kind() == reflect.Interface {
		// Otherwise, search for a type that implements the interface

		for t, reg := range c.services {
			if t.Implements(svcType) {
				svc, err = c.callFactory(reg)
			}
		}
	}

	if err != emptyValue {
		return emptyValue, err
	}

	if svc != emptyValue {
		if pointerVal.Elem() != reflect.ValueOf(nil) {
			pointerVal.Elem().Set(svc)
		}

		return svc, emptyValue
	}

	return emptyValue, reflect.ValueOf(ErrServiceNotRegistered)
}

func (c *Container) callFactory(reg *serviceRegistration) (reflect.Value, reflect.Value) {
	factoryType := reg.factory.Type()

	args := make([]reflect.Value, factoryType.NumIn())
	for i := 0; i < len(args); i++ {
		argType := factoryType.In(i)

		val, err := c.getInstance(reflect.New(argType).Interface())
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

	return out[0], err
}
