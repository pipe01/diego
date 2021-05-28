# diego

diego is a simple dependency injection container for Go. It currently supports singleton and transient lifetimes.

Example usage:

```go
func main() {
	c := diego.NewContainer()

	c.Register(NewA, diego.Transient)
	c.Register(NewB, diego.Transient)

	var b IntB
	c.MustGetInstance(&b)

	b.B()
}

type IntA interface {
	A()
}

type ImplA struct{}

func (*ImplA) A() {}

func NewA() *ImplA { return &ImplA{} }

type IntB interface {
	B()
}

type ImplB struct{ a IntA }

func (*ImplB) B() {}

func NewB(a IntA) *ImplB { return &ImplB{a} }

```
