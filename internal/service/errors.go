package service

import (
	"errors"
	"fmt"
)

// Kind classifies a failure so that the HTTP and MCP layers can each translate
// it once, in one place, instead of inspecting error strings.
type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindUnauthenticated
	KindForbidden
	KindNotFound
	KindConflict
)

func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "invalid"
	case KindUnauthenticated:
		return "unauthenticated"
	case KindForbidden:
		return "forbidden"
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	default:
		return "internal"
	}
}

// Error is what the service layer returns. The message is written to be read by
// an agent as well as a person: it says what happened and what to do instead.
type Error struct {
	Kind Kind
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e.Msg == "" && e.Err != nil {
		return e.Err.Error()
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

func invalid(format string, args ...any) *Error {
	return &Error{Kind: KindInvalid, Msg: fmt.Sprintf(format, args...)}
}

func forbidden(format string, args ...any) *Error {
	return &Error{Kind: KindForbidden, Msg: fmt.Sprintf(format, args...)}
}

func notFound(format string, args ...any) *Error {
	return &Error{Kind: KindNotFound, Msg: fmt.Sprintf(format, args...)}
}

func conflict(format string, args ...any) *Error {
	return &Error{Kind: KindConflict, Msg: fmt.Sprintf(format, args...)}
}

func unauthenticated(format string, args ...any) *Error {
	return &Error{Kind: KindUnauthenticated, Msg: fmt.Sprintf(format, args...)}
}

func internal(err error) *Error {
	return &Error{Kind: KindInternal, Msg: "internal error", Err: err}
}

// KindOf reports the classification of any error the service returned.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}
