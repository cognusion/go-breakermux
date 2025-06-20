

# breakermux
`import "github.com/cognusion/go-breakermux"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)
* [Examples](#pkg-examples)

## <a name="pkg-overview">Overview</a>
Package breakermux builds upon gobreaker (<a href="https://github.com/sony/gobreaker/">https://github.com/sony/gobreaker/</a>),
allowing for circuitbreakers to be automatically instantiated for different keys.
This could be used to 'break on URLs, hostnames, service descriptions, etc.


##### Example :
``` go
// Print something nice when the state changes
var st = Settings[string]{}
st.OnStateChange = func(name string, from gobreaker.State, to gobreaker.State) {
    fmt.Printf("%s: %+v -> %+v\n", name, from, to)
}

// Our ExecFunc sleeps for a half-second so it is visually obvious when it is being
// called ('breaker closed), versus skipped ('breaker open)
st.ExecClosure = func(input string) func() (string, error) {
    return func() (string, error) {
        time.Sleep(500 * time.Millisecond)
        if input == "yes" {
            return "yes", nil
        }
        return "no", fmt.Errorf("Noo")
    }
}

// Create a mux, passing it our Settings and ExecFunc
cbm := NewMux(st)

// Always clean up after ourselves!
defer cbm.Close()

// We call "no" 20 times, but after the first 5 it will trip and fast-fail the last 15.
for i := range 20 {
    fmt.Printf("%d: ", i)
    cbm.Get("no")
}
fmt.Println()

// We call "yes" 20 times, and each will take a. half. second. to. return.
for i := range 20 {
    fmt.Printf("%d: ", i)
    cbm.Get("yes")
}
fmt.Println()
```



## <a name="pkg-index">Index</a>
* [type CircuitBreakerMux](#CircuitBreakerMux)
  * [func NewMux[T any](st Settings[T]) *CircuitBreakerMux[T]](#NewMux)
  * [func (c *CircuitBreakerMux[T]) Clear()](#CircuitBreakerMux.Clear)
  * [func (c *CircuitBreakerMux[T]) Close()](#CircuitBreakerMux.Close)
  * [func (c *CircuitBreakerMux[T]) Delete(key string)](#CircuitBreakerMux.Delete)
  * [func (c *CircuitBreakerMux[T]) Get(key string) (value T, err error)](#CircuitBreakerMux.Get)
  * [func (c *CircuitBreakerMux[T]) GetKeyExec(key, exec string) (value T, err error)](#CircuitBreakerMux.GetKeyExec)
* [type ExecFunc](#ExecFunc)
* [type Settings](#Settings)

#### <a name="pkg-examples">Examples</a>
* [Package](#example-)

#### <a name="pkg-files">Package files</a>
[cbmux.go](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go)






## <a name="CircuitBreakerMux">type</a> [CircuitBreakerMux](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go?s=719:853#L21)
``` go
type CircuitBreakerMux[T any] struct {
    // contains filtered or unexported fields
}

```
CircuitBreakerMux is a goro-safe circuit breaker multiplex,
whereby individual keys gets their own 'breakers,
which can each be in various states. They must all share a return type.

In general, an implementation should use either Get(.) or GetKeyExec(..) depending
on the specificity of the executing request versus the granularity of the desired
circuit.







### <a name="NewMux">func</a> [NewMux](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go?s=911:967#L29)
``` go
func NewMux[T any](st Settings[T]) *CircuitBreakerMux[T]
```
NewMux requires a Settings for proper configuration.





### <a name="CircuitBreakerMux.Clear">func</a> (\*CircuitBreakerMux[T]) [Clear](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go?s=3496:3534#L135)
``` go
func (c *CircuitBreakerMux[T]) Clear()
```
Clear removes all keys and 'breakers.




### <a name="CircuitBreakerMux.Close">func</a> (\*CircuitBreakerMux[T]) [Close](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go?s=1761:1799#L70)
``` go
func (c *CircuitBreakerMux[T]) Close()
```
Close is the proper way to stop using a mux.




### <a name="CircuitBreakerMux.Delete">func</a> (\*CircuitBreakerMux[T]) [Delete](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go?s=3376:3425#L130)
``` go
func (c *CircuitBreakerMux[T]) Delete(key string)
```
Delete removes a 'breaker named by key, if one exists.




### <a name="CircuitBreakerMux.Get">func</a> (\*CircuitBreakerMux[T]) [Get](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go?s=2017:2084#L77)
``` go
func (c *CircuitBreakerMux[T]) Get(key string) (value T, err error)
```
Get fetches an existing 'breaker for the key, or creates a new one,
executes the ExecFunc on it, and returns accordingly.




### <a name="CircuitBreakerMux.GetKeyExec">func</a> (\*CircuitBreakerMux[T]) [GetKeyExec](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go?s=2728:2808#L104)
``` go
func (c *CircuitBreakerMux[T]) GetKeyExec(key, exec string) (value T, err error)
```
GetKeyExec fetches an existing 'breaker for the key, or creates a new one,
passing exec to the ExecFunc, and returns accordingly.




## <a name="ExecFunc">type</a> [ExecFunc](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go?s=4215:4266#L160)
``` go
type ExecFunc[T any] func(string) func() (T, error)
```
ExecFunc is a closure to allow a string to be passed to an otherwise niladic function.










## <a name="Settings">type</a> [Settings](https://github.com/cognusion/go-breakermux/tree/master/cbmux.go?s=6353:6502#L201)
``` go
type Settings[T any] struct {
    gobreaker.Settings
    ExecClosure func(string) func() (T, error)
    ExpireAfter time.Duration
    ExpireCheck time.Duration
}

```
Settings allows for per-mux and per-'breaker configurations. Changing values after passing it to
NewMux is undefined.

Name is the name of the CircuitBreaker. This is overridden per-'breaker.

MaxRequests is the maximum number of requests allowed to pass through
when the CircuitBreaker is half-open.
If MaxRequests is 0, the CircuitBreaker allows only 1 request.

Interval is the cyclic period of the closed state
for the CircuitBreaker to clear the internal Counts.
If Interval is less than or equal to 0, the CircuitBreaker doesn't clear internal Counts during the closed state.

Timeout is the period of the open state,
after which the state of the CircuitBreaker becomes half-open.
If Timeout is less than or equal to 0, the timeout value of the CircuitBreaker is set to 60 seconds.

ReadyToTrip is called with a copy of Counts whenever a request fails in the closed state.
If ReadyToTrip returns true, the CircuitBreaker will be placed into the open state.
If ReadyToTrip is nil, default ReadyToTrip is used.
Default ReadyToTrip returns true when the number of consecutive failures is more than 5.

OnStateChange is called whenever the state of the CircuitBreaker changes.

IsSuccessful is called with the error returned from a request.
If IsSuccessful returns true, the error is counted as a success.
Otherwise the error is counted as a failure.
If IsSuccessful is nil, default IsSuccessful is used, which returns false for all non-nil errors.

ExecClosure is a closure to allow a string to be passed to an otherwise niladic function for execution by
the 'breakers.

ExpireAfter is a Duration after which an unused 'breaker may be removed for the mux.
Overly brief expirations are not advised.
If ExpireAfter is less than or equal to 0, all 'breakers will be removed every ExpireCheck.

ExpireCheck is an interval when expiration checks will be performed.
Overly aggressive expiration is not advised.
If ExpireCheck is less than or equal to 0, expiration will not occur.














- - -
Generated by [godoc2md](http://github.com/cognusion/godoc2md)
