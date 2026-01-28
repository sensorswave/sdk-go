package sensorswave

import "errors"

var (
	// This error is returned by methods of the `Client` interface when they are
	// called after the client was already closed.
	ErrClosed = errors.New("the client was already closed")

	// This error is used to notify the application that too many requests are
	// already being sent and no more messages can be accepted.
	ErrTooManyRequests = errors.New("too many requests are already in-flight")
	ErrInvalidResponse = errors.New("invalid response from server")

	// This error is used to notify the client callbacks that a message send
	// failed because the JSON representation of a message exceeded the upper
	// limit.
	ErrMessageTooBig = errors.New("the message batch exceeds the maximum allowed http body size")

	ErrEventNameEmpty          = errors.New("event name is empty")
	ErrEventNameTooLong        = errors.New("event name is too long, >128")
	ErrPropertyKeyTooLong      = errors.New("property key is too long, >128")
	ErrEmptyUserIDs            = errors.New("login_id and anon_id are both empty")
	ErrIdentifyRequiredBothIDs = errors.New("Identify requires both login_id and anon_id to be non-empty")

	//
	ErrABNotInited     = errors.New("ab core not inited")
	ErrABNotReady      = errors.New("ab core not ready")
	ErrABInvalidKey    = errors.New("ab key is invalid")
	ErrABWithoutSticky = errors.New("ab need sticky handler but not set")
)
