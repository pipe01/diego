package diego

// Lifetime indicates how long a service lives and how often it is created.
type Lifetime byte

const (
	// Transient services are instantiated every time they are requested.
	Transient Lifetime = iota

	// Singleton services are instantiated only once, and reused on future requests.
	Singleton

	// Scoped services are instantiated once per scope.
	//Scoped
)
