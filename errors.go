package diego

import "fmt"

// ErrorNotRegistered is returned if a requested service is not found.
type ErrorNotRegistered struct {
	RequestedType string
}

func (e *ErrorNotRegistered) Error() string {
	return fmt.Sprintf("service %s not registered", e.RequestedType)
}
