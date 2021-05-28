package diego

import (
	"fmt"
	"strings"
)

// ErrorNotRegistered is returned if a requested service is not found.
type ErrorNotRegistered struct {
	RequestedType string
}

func (e *ErrorNotRegistered) Error() string {
	return fmt.Sprintf("service %s not registered", e.RequestedType)
}

// ErrorCircularDependency is returned if a circular dependency between services is detected.
type ErrorCircularDependency struct {
	Types []string
}

func (e *ErrorCircularDependency) Error() string {
	return "circular dependency detected: " + strings.Join(e.Types, " -> ")
}
