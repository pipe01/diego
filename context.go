package diego

import (
	"reflect"
)

type context struct {
	order         []reflect.Type
	instantiating map[reflect.Type]int
}

func newContext() *context {
	return &context{
		instantiating: map[reflect.Type]int{},
	}
}

func (c *context) isInstantiating(t reflect.Type) bool {
	_, ok := c.instantiating[t]
	return ok
}

func (c *context) push(t reflect.Type) bool {
	isValid := !c.isInstantiating(t)

	c.order = append(c.order, t)
	c.instantiating[t] = len(c.order) - 1
	return isValid
}

func (c *context) pop() {
	t := c.order[len(c.order)-1]

	delete(c.instantiating, t)
	c.order = c.order[:len(c.order)-1]
}

func (c *context) typeNames() (names []string) {
	names = make([]string, len(c.order))

	for i, t := range c.order {
		names[i] = t.Name()
	}

	return
}
