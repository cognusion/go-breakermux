// Package breakermux builds upon gobreaker (https://github.com/sony/gobreaker/),
// allowing for circuitbreakers to be automatically instantiated for different keys.
// This could be used to 'break on URLs, hostnames, service descriptions, etc.
package breakermux

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/sony/gobreaker/v2"
)

// CircuitBreakerMux is a goro-safe circuit breaker multiplex,
// whereby individual keys gets their own 'breakers,
// which can each be in various states. They must all share a return type.
//
// In general, an implementation should use either Get(.) or GetKeyExec(..) depending
// on the specificity of the executing request versus the granularity of the desired
// circuit.
type CircuitBreakerMux[T any] struct {
	breakers sync.Map
	st       gobreaker.Settings
	efunc    ExecFunc[T]
	killChan chan struct{}
}

// NewMux requires a Settings for proper configuration.
func NewMux[T any](st Settings[T]) *CircuitBreakerMux[T] {
	var c CircuitBreakerMux[T]
	c.killChan = make(chan struct{})

	if st.ExpireCheck > 0 {
		if st.ExpireAfter <= 0 {
			// clear the map each interval
			go func() {
				for {
					select {
					case <-c.killChan:
						return
					case <-time.After(st.ExpireCheck):
						// We use high-level Clear(), instead of
						// breakers.Clear(), so we can possibly [re]set
						// other things in the future.
						c.Clear()
					}
				}
			}()
		} else {
			// traverse the map each interval
			go func() {
				for {
					select {
					case <-c.killChan:
						return
					case <-time.After(st.ExpireCheck):
						c.expire(time.Now().Add(st.ExpireAfter * -1))
					}
				}
			}()
		}
	}

	c.st = st.Settings
	c.efunc = st.ExecClosure
	return &c
}

// Close is the proper way to stop using a mux.
func (c *CircuitBreakerMux[T]) Close() {
	close(c.killChan)
	c.breakers.Clear() // low-level Clear() to avoid state changes.
}

// Get fetches an existing 'breaker for the key, or creates a new one,
// executes the ExecFunc on it, and returns accordingly.
func (c *CircuitBreakerMux[T]) Get(key string) (value T, err error) {
	if cba, ok := c.breakers.Load(key); ok {
		// Got one!
		var cb = cba.(*cache).Get().(*gobreaker.CircuitBreaker[T])
		value, err = cb.Execute(c.efunc(key))
	} else {
		// Need a new one!
		// Clone the default settings, update the name
		var ust = c.st
		ust.Name = key

		// Create the cb, set it in the map
		cb := gobreaker.NewCircuitBreaker[T](ust)

		var cba cache
		cba.New(cb)
		c.breakers.Store(key, &cba)

		// Go for it!
		value, err = cb.Execute(c.efunc(key))
	}

	return value, err
}

// GetKeyExec fetches an existing 'breaker for the key, or creates a new one,
// passing exec to the ExecFunc, and returns accordingly.
func (c *CircuitBreakerMux[T]) GetKeyExec(key, exec string) (value T, err error) {
	if cba, ok := c.breakers.Load(key); ok {
		// Got one!
		var cb = cba.(*cache).Get().(*gobreaker.CircuitBreaker[T])
		value, err = cb.Execute(c.efunc(exec))
	} else {
		// Need a new one!
		// Clone the default settings, update the name
		var ust = c.st
		ust.Name = key

		// Create the cb, set it in the map
		cb := gobreaker.NewCircuitBreaker[T](ust)

		var cba cache
		cba.New(cb)
		c.breakers.Store(key, &cba)

		// Go for it!
		value, err = cb.Execute(c.efunc(exec))
	}

	return value, err
}

// Delete removes a 'breaker named by key, if one exists.
func (c *CircuitBreakerMux[T]) Delete(key string) {
	c.breakers.Delete(key)
}

// Clear removes all keys and 'breakers.
func (c *CircuitBreakerMux[T]) Clear() {
	c.breakers.Clear()
}

// expire is a private method to traverse the mux and remove any objects
// where the atime is before deadtime.
func (c *CircuitBreakerMux[T]) expire(deadtime time.Time) {
	deadint := deadtime.UnixMicro()

	// As a rule, we only operate on c.breakers directly here to prevent
	// future oopsies with higher-level functions that may change state.

	// Range(f func(key, value any) bool)
	f := func(key, value any) bool {
		atime := value.(*cache).atime.Load()
		if atime < deadint {
			c.breakers.Delete(key)
		}
		return true
	}

	c.breakers.Range(f)
}

// ExecFunc is a closure to allow a string to be passed to an otherwise niladic function.
type ExecFunc[T any] func(string) func() (T, error)

// Settings allows for per-mux and per-'breaker configurations. Changing values after passing it to
// NewMux is undefined.
//
// Name is the name of the CircuitBreaker. This is overridden per-'breaker.
//
// MaxRequests is the maximum number of requests allowed to pass through
// when the CircuitBreaker is half-open.
// If MaxRequests is 0, the CircuitBreaker allows only 1 request.
//
// Interval is the cyclic period of the closed state
// for the CircuitBreaker to clear the internal Counts.
// If Interval is less than or equal to 0, the CircuitBreaker doesn't clear internal Counts during the closed state.
//
// Timeout is the period of the open state,
// after which the state of the CircuitBreaker becomes half-open.
// If Timeout is less than or equal to 0, the timeout value of the CircuitBreaker is set to 60 seconds.
//
// ReadyToTrip is called with a copy of Counts whenever a request fails in the closed state.
// If ReadyToTrip returns true, the CircuitBreaker will be placed into the open state.
// If ReadyToTrip is nil, default ReadyToTrip is used.
// Default ReadyToTrip returns true when the number of consecutive failures is more than 5.
//
// OnStateChange is called whenever the state of the CircuitBreaker changes.
//
// IsSuccessful is called with the error returned from a request.
// If IsSuccessful returns true, the error is counted as a success.
// Otherwise the error is counted as a failure.
// If IsSuccessful is nil, default IsSuccessful is used, which returns false for all non-nil errors.
//
// ExecClosure is a closure to allow a string to be passed to an otherwise niladic function for execution by
// the 'breakers.
//
// ExpireAfter is a Duration after which an unused 'breaker may be removed for the mux.
// Overly brief expirations are not advised.
// If ExpireAfter is less than or equal to 0, all 'breakers will be removed every ExpireCheck.
//
// ExpireCheck is an interval when expiration checks will be performed.
// Overly aggressive expiration is not advised.
// If ExpireCheck is less than or equal to 0, expiration will not occur.
type Settings[T any] struct {
	gobreaker.Settings
	ExecClosure func(string) func() (T, error)
	ExpireAfter time.Duration
	ExpireCheck time.Duration
}

// cache is an internal-only storable, that when used properly allows for fast
// access tracking of write-once, read-many cache objects.
//
// The *time members should never be inspected nor set outside of atomic operations.
// UnixMicro is used. If sub-microsecond resolution is important
// (e.g. if you're expiring below or near microsecond intervals for some reason),
// change the various instances below, and the 'deadtime' resolution in func expire() above.
type cache struct {
	item  any
	atime atomic.Int64
	mtime atomic.Int64
}

// New sets atime and mtime, ad stores the item.
// One *could* use this to store a different item in the same cache,
// but one *should not*: Abandon this cache and create a new one.
func (c *cache) New(item any) {
	c.mtime.Store(time.Now().UnixMicro())
	c.atime.Store(time.Now().UnixMicro())
	c.item = item
}

// Get updates atime, and returns the item.
func (c *cache) Get() any {
	c.atime.Store(time.Now().UnixMicro())
	return c.item
}

// Atime returns the a(ccess) time as a Time.
func (c *cache) Atime() time.Time {
	return time.UnixMicro(c.atime.Load())
}

// Mtime returns the m(odification) time as a Time.
func (c *cache) Mtime() time.Time {
	return time.UnixMicro(c.mtime.Load())
}
