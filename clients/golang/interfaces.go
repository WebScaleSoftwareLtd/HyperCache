package hypercache

// BaseImplementation is implementation functionality used by both HTTP and HNP.
type BaseImplementation interface {
	// Ping is used to ping the server.
	Ping() error

	// Get is used to get a record.
	Get(key []byte) ([]byte, error)
}

// HNPImplementation includes HNP exclusive functionality.
type HNPImplementation interface {
	BaseImplementation

	// AddEventHandler is used to add a handler for custom events.
	// Note that the bytes should not be mutated.
	AddEventHandler(ch chan []byte)

	// MutexLock is used to lock a global mutex.
	MutexLock() error

	// MutexUnlock is used to unlock a globally locked mutex.
	MutexUnlock() error

	// SendEvent is used to send an event to the HyperCache server.
	SendEvent(b []byte) error
}
