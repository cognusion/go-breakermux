// Package breakermux builds upon gobreaker (https://github.com/sony/gobreaker/),
// allowing for circuitbreakers to be automatically instantiated for different keys.
// This could be used to 'break on URLs or hostnames, etc.
//
// Plans include expiry of 'breakers, hence the tracking of mtime and atime.
package breakermux

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/sony/gobreaker/v2"
)

// CircuitBreakerMux is a goro-safe circuit breaker multiplex,
// whereby individual keys gets their own 'breakers,
// which can each be in various states.
type CircuitBreakerMux[T any] struct {
	breakers sync.Map
	st       gobreaker.Settings
	efunc    ExecFunc[T]
}

// NewMux requires a Settings for proper configuration.
func NewMux[T any](st Settings[T]) *CircuitBreakerMux[T] {
	var c CircuitBreakerMux[T]
	c.st = st.Settings
	c.efunc = st.ExecClosure
	return &c
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
		ust := c.st
		ust.Name = key

		// Create the cb, set it in the map
		cb := gobreaker.NewCircuitBreaker[T](ust)

		var cba = newCache()
		cba.New(cb)
		c.breakers.Store(key, &cba)

		// Go for it!
		value, err = cb.Execute(c.efunc(key))
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

// ExecFunc is a closure to allow a string to be passed to an otherwise niladic function.
type ExecFunc[T any] func(string) func() (T, error)

// Settings allows for per-mux and per-'breaker configurations.
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
// If ExpireAfter is less than or equal to 0, expiration will not occur.
type Settings[T any] struct {
	gobreaker.Settings
	ExecClosure func(string) func() (T, error)
	ExpireAfter time.Duration
}

type cache struct {
	item  any
	atime *int64
	mtime *int64
}

func newCache() cache {
	return cache{
		atime: new(int64),
		mtime: new(int64),
	}
}

func (c *cache) New(item any) {
	atomic.StoreInt64(c.mtime, time.Now().UnixMicro())
	atomic.StoreInt64(c.atime, time.Now().UnixMicro())
	c.item = item
}

func (c *cache) Get() any {
	atomic.StoreInt64(c.atime, time.Now().UnixMicro())
	return c.item
}

func (c *cache) Atime() time.Time {
	return time.UnixMicro(atomic.LoadInt64(c.atime))
}

func (c *cache) Mtime() time.Time {
	return time.UnixMicro(atomic.LoadInt64(c.mtime))
}
