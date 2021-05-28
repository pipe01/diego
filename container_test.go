package diego

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterInvalid(t *testing.T) {
	c := NewContainer()

	t.Run("instance not singleton", func(t *testing.T) {
		assert.PanicsWithValue(t, panicInstanceLifetimeSingleton, func() {
			c.Register(0, Transient)
		})
	})

	t.Run("too many factory outputs", func(t *testing.T) {
		assert.PanicsWithValue(t, panicFactoryMethodOutputs, func() {
			c.Register(func() (int, int, int) { return 0, 0, 0 }, Transient)
		})
	})

	t.Run("no factory outputs", func(t *testing.T) {
		assert.PanicsWithValue(t, panicFactoryMethodOutputs, func() {
			c.Register(func() {}, Transient)
		})
	})

	t.Run("second factory output not error", func(t *testing.T) {
		assert.PanicsWithValue(t, panicFactorySecondOutputError, func() {
			c.Register(func() (int, int) { return 0, 0 }, Transient)
		})
	})

	t.Run("already registered", func(t *testing.T) {
		c.Register(123, Singleton)

		assert.Panics(t, func() {
			c.Register(123, Singleton)
		})
	})
}

func TestGetInvalid(t *testing.T) {
	c := NewContainer()

	t.Run("is nil", func(t *testing.T) {
		assert.PanicsWithValue(t, panicGetInstanceNil, func() {
			c.GetInstance(nil)
		})
	})

	t.Run("not pointer", func(t *testing.T) {
		assert.PanicsWithValue(t, panicParamIsPointer, func() {
			c.GetInstance(123)
		})
	})

	t.Run("factory error", func(t *testing.T) {
		err := errors.New("test")

		c.Register(func() (int, error) { return 0, err }, Singleton)

		_, retErr := c.GetInstance((*int)(nil))

		assert.ErrorIs(t, retErr, err)
	})
}

func TestSimpleRegisterAndGet(t *testing.T) {
	c := NewContainer()

	val := 123

	c.Register(val, Singleton)

	var ret int
	c.GetInstance(&ret)

	assert.Equal(t, val, ret)
}

type testStruct struct{}

func (*testStruct) Test() {}

func TestInterfaceRegisterAndGet(t *testing.T) {
	c := NewContainer()

	inst := &testStruct{}

	c.Register(inst, Singleton)

	var ret interface{ Test() }
	_, err := c.GetInstance(&ret)

	assert.Nil(t, err)
	assert.Equal(t, inst, ret)
}

type structA struct{}
type structB struct{}
type structC struct{}

func TestCircularDependency(t *testing.T) {
	c := NewContainer()

	c.Register(func(structC) structA { return structA{} }, Singleton)
	c.Register(func(structA) structB { return structB{} }, Singleton)
	c.Register(func(structB) structC { return structC{} }, Singleton)

	_, err := c.GetInstance((*structC)(nil))

	assert.NotNil(t, err)
}

func TestCircularDependencyFunc(t *testing.T) {
	c := NewContainer()

	c.Register(func(structC) structA { return structA{} }, Singleton)
	c.Register(func(structA) structB { return structB{} }, Singleton)
	c.Register(func(func() structB) structC { return structC{} }, Singleton)

	_, err := c.GetInstance((*structC)(nil))

	assert.Nil(t, err)
}
