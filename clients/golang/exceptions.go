package hypercache

// ClientError is the base error for client errors. All other returned errors
// wrap this.
type ClientError struct {
	description []byte
}

func (e ClientError) Error() string {
	return string(e.description)
}

type clientErrorWrapper struct {
	description []byte
}

func (e clientErrorWrapper) Error() string {
	return string(e.description)
}

func (e clientErrorWrapper) Unwrap() error {
	return ClientError{description: e.description}
}

// InvalidPacket is returned by the server when it deems a packet as invalid.
type InvalidPacket struct {
	clientErrorWrapper
}

// NotFound is returned when the record is not found.
type NotFound struct {
	clientErrorWrapper
}

// UnlockError is returned when a mutex is unable to be unlocked.
type UnlockError struct {
	clientErrorWrapper
}

// InvalidCredentials is returned when the users credentials are invalid.
type InvalidCredentials struct {
	clientErrorWrapper
}

// DatabaseNotFound is thrown when the database specified is not found.
type DatabaseNotFound struct {
	clientErrorWrapper
}

var errFactories = map[string]func([]byte) error{
	"InvalidPacket": func(b []byte) error {
		return InvalidPacket{clientErrorWrapper{b}}
	},
	"NotFound": func(b []byte) error {
		return NotFound{clientErrorWrapper{b}}
	},
	"UnlockError": func(b []byte) error {
		return UnlockError{clientErrorWrapper{b}}
	},
	"InvalidCredentials": func(b []byte) error {
		return InvalidCredentials{clientErrorWrapper{b}}
	},
	"DatabaseNotFound": func(b []byte) error {
		return DatabaseNotFound{clientErrorWrapper{b}}
	},
}

func toException(exceptionName string, exceptionDescriptionB []byte) error {
	factory, ok := errFactories[exceptionName]
	if !ok {
		return ClientError{description: exceptionDescriptionB}
	}
	return factory(exceptionDescriptionB)
}
