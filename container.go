package diego

import (
	"errors"
	"fmt"
	"io"
	"reflect"
)

const (
	panicInstanceLifetimeSingleton = "lifetime must be singleton when registering an instance"
	panicFactoryMethodOutputs      = "factory method must have either 1 or 2 outputs"
	panicFactorySecondOutputError  = "factory method's second output must be an error"

	panicFnIsFunc         = "fn must be a func"
	panicFnSingleInput    = "fn must have a single input"
	panicFnInputInterface = "fn's input must be an interface type"

	panicGetInstanceNil = "nil value passed to GetInstance. Make sure you pass a pointer, not an interface value"
	panicParamIsPointer = "type parameter must be a pointer"
)

var emptyValue = reflect.Value{}

type serviceRegistration struct {
	serviceType reflect.Type
	factory     reflect.Value
	lifetime    Lifetime
	instance    interface{}
}

// Container holds a collection of services
type Container struct {
	services map[reflect.Type]*serviceRegistration
}

// NewContainer creates a new empty container
func NewContainer() *Container {
	return &Container{
		services: map[reflect.Type]*serviceRegistration{},
	}
}

// Register registers a service implementation in the container with a lifetime.
//
// If v is a function, it must have either one or two outputs.
// The first one will be the service implementation that will be registered,
// and the second one, if present, must be an error type.
//
// If v is not a function, it will be directly registered as a singleton instance.
func (c *Container) Register(v interface{}, lifetime Lifetime) {
	t := reflect.TypeOf(v)

	if t.Kind() != reflect.Func {
		if lifetime != Singleton {
			panic(panicInstanceLifetimeSingleton)
		}
		c.registerInstance(v, lifetime)
		return
	}

	if t.NumOut() != 1 && t.NumOut() != 2 {
		panic(panicFactoryMethodOutputs)
	}

	if t.NumOut() == 2 && !t.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		panic(panicFactorySecondOutputError)
	}

	svcType := t.Out(0)

	if _, ok := c.services[svcType]; ok {
		panic(fmt.Sprintf("type %s is already registered", svcType.Name()))
	}

	c.services[svcType] = &serviceRegistration{
		serviceType: svcType,
		factory:     reflect.ValueOf(v),
		lifetime:    lifetime,
	}
}

func (c *Container) registerInstance(inst interface{}, lifetime Lifetime) {
	svcType := reflect.TypeOf(inst)

	if _, ok := c.services[svcType]; ok {
		panic(fmt.Sprintf("type %s already registered", svcType.Name()))
	}

	c.services[svcType] = &serviceRegistration{
		serviceType: svcType,
		instance:    inst,
		lifetime:    lifetime,
	}
}

// Close calls Close() on all singleton instances that implement it.
func (c *Container) Close() {
	for _, r := range c.services {
		if r.instance != nil {
			closer, ok := r.instance.(io.Closer)
			if ok {
				closer.Close()
			}
		}
	}
}

// All takes a function with a single input of an interface type and calls it for every service that implements that interface.
func (c *Container) All(fn interface{}) {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnValue.Kind() != reflect.Func {
		panic(panicFnIsFunc)
	}
	if fnType.NumIn() != 1 {
		panic(panicFnSingleInput)
	}
	if fnType.In(0).Kind() != reflect.Interface {
		panic(panicFnInputInterface)
	}

	in := fnType.In(0)

	for t, reg := range c.services {
		if t.Implements(in) {
			svc, err := c.instantiate(nil, reg)
			if err != emptyValue && !err.IsNil() {
				panic(err.Interface())
			}

			fnValue.Call([]reflect.Value{svc})
		}
	}
}

// GetInstance takes a pointer to any of the following types:
//
// - Func: it must have no inputs and one output, a func will be returned that will return the service when called.
//
// - Interface: if no type is registered that directly matches this type, the first registered type that implements
// this interface will be returned.
//
// - Any other value: the type will be looked up as-is.
func (c *Container) GetInstance(tp interface{}) (interface{}, error) {
	if tp == nil {
		panic(panicGetInstanceNil)
	}

	pointer := reflect.TypeOf(tp)
	pointerVal := reflect.ValueOf(tp)

	if pointer.Kind() != reflect.Ptr {
		panic(panicParamIsPointer)
	}

	val, err := c.getInstance(nil, pointer.Elem())
	if err != emptyValue && !err.IsNil() {
		errVal := err.Interface().(error)

		var circularErr *ErrorCircularDependency
		if errors.As(errVal, &circularErr) {
			return nil, circularErr
		}

		return nil, errVal
	}

	if pointerVal.Elem() != reflect.ValueOf(nil) {
		pointerVal.Elem().Set(val)
	}

	return val.Interface(), nil
}

// MustGetInstance is like GetInstance, but panics if an error is returned.
func (c *Container) MustGetInstance(tp interface{}) interface{} {
	v, err := c.GetInstance(tp)
	if err != nil {
		panic(err)
	}
	return v
}

// Call takes a function with any inputs and calls it, filling the inputs with values from the container.
func (c *Container) Call(fn interface{}) {
	val := reflect.ValueOf(fn)
	typ := val.Type()

	if val.Kind() != reflect.Func {
		panic(panicFnIsFunc)
	}

	args := make([]reflect.Value, typ.NumIn())
	for i := 0; i < len(args); i++ {
		argType := typ.In(i)
		argVal, err := c.getInstance(nil, argType)
		if err != emptyValue && !err.IsNil() {
			panic(err.Interface())
		}

		args[i] = argVal
	}

	val.Call(args)
}

func (c *Container) getInstance(ctx *context, svcType reflect.Type) (svc reflect.Value, err reflect.Value) {
	// If the type is directly registered, call its factory
	if reg, ok := c.services[svcType]; ok {
		svc, err = c.instantiate(ctx, reg)
	} else if svcType.Kind() == reflect.Interface {
		// Otherwise, search for a type that implements the interface

		for t, reg := range c.services {
			if t.Implements(svcType) {
				svc, err = c.instantiate(nil, reg)
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
			svc, err := c.getInstance(ctx, svcType)
			if err != emptyValue && !err.IsNil() {
				panic(err.Interface())
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

func (c *Container) instantiate(ctx *context, reg *serviceRegistration) (reflect.Value, reflect.Value) {
	if ctx == nil {
		ctx = newContext()
	}

	if !ctx.push(reg.serviceType) {
		return emptyValue, reflect.ValueOf(&ErrorCircularDependency{ctx.typeNames()})
	}
	defer ctx.pop()

	if reg.instance != nil {
		return reflect.ValueOf(reg.instance), emptyValue
	}

	factoryType := reg.factory.Type()

	args := make([]reflect.Value, factoryType.NumIn())
	for i := 0; i < len(args); i++ {
		argType := factoryType.In(i)

		val, err := c.getInstance(ctx, argType)
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
