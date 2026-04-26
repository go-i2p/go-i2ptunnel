package i2ptunnel

import "sync"

// MaxErrors is the maximum number of errors retained in an ErrorTracker.
// Oldest errors are discarded once this limit is reached.
const MaxErrors = 100

// ErrorTracker is an embeddable type that provides bounded, thread-safe error
// history for I2P tunnel implementations. All tunnel types embed this struct
// to avoid duplicating the recordError / Error() / ErrorHistory() boilerplate.
type ErrorTracker struct {
	// Errors is the bounded error history. Access via Record/Last/All for
	// thread-safe operations. Direct field access is available within the
	// same package for backwards-compatible test helpers.
	Errors []I2PTunnelError
	mu     sync.Mutex
}

// Record appends err to the error history. When the history exceeds MaxErrors,
// the oldest entries are discarded to bound memory usage. owner must implement
// Name() string; any I2PTunnel or *TunnelBase value satisfies this.
func (et *ErrorTracker) Record(owner interface{ Name() string }, err error) {
	et.mu.Lock()
	et.Errors = append(et.Errors, NewError(owner, err))
	if len(et.Errors) > MaxErrors {
		et.Errors = append([]I2PTunnelError(nil), et.Errors[len(et.Errors)-MaxErrors:]...)
	}
	et.mu.Unlock()
}

// Last returns the most recently recorded error, or nil if none have been
// recorded. The returned value implements the error interface.
func (et *ErrorTracker) Last() error {
	et.mu.Lock()
	defer et.mu.Unlock()
	if len(et.Errors) > 0 {
		return et.Errors[len(et.Errors)-1]
	}
	return nil
}

// All returns a thread-safe snapshot copy of all recorded errors. Returns nil
// when no errors have been recorded.
func (et *ErrorTracker) All() []I2PTunnelError {
	et.mu.Lock()
	defer et.mu.Unlock()
	if len(et.Errors) == 0 {
		return nil
	}
	out := make([]I2PTunnelError, len(et.Errors))
	copy(out, et.Errors)
	return out
}
