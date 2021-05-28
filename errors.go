package diego

import "fmt"

type ErrorNotRegistered struct {
	RequestedType string
}

func (e *ErrorNotRegistered) Error() string {
	return fmt.Sprintf("service %s not registered", e.RequestedType)
}
