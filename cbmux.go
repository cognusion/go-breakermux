// Package breakermux builds upon [gobreaker](https://github.com/sony/gobreaker/v2/),
// allowing for circuitbreakers to be automatically instantiated for different keys.
// This could be used to 'break on URLs or hostnames, etc.
package breakermux

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/sony/gobreaker/v2"
)

// DefaultSettings is exactly that, so implementors need not import
// gobreaker directly.
var DefaultSettings gobreaker.Settings

// CircuitBreakerMux is a goro-safe circuit breaker multiplex,
// whereby individual keys gets their own 'breakers,
// which can each be in various states.
type CircuitBreakerMux[T any] struct {
	breakers sync.Map
	st       gobreaker.Settings
	efunc    ExecFunc[T]
}

// NewMux requires a Settings struct to use for each 'breaker, and an ExecFunc that each will use.
func NewMux[T any](st gobreaker.Settings, execfunc ExecFunc[T]) *CircuitBreakerMux[T] {
	var c CircuitBreakerMux[T]
	c.st = st
	c.efunc = execfunc
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
