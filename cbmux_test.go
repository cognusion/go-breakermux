package breakermux

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/sony/gobreaker/v2"
)

func Example() {

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
}

func TestCache(t *testing.T) {

	Convey("When a new cache is created, the times are correct, the item is correct, and atime was updated.", t, func() {
		oldtime := time.Now()
		time.Sleep(1 * time.Millisecond)

		var cba = newCache()
		cba.New("Hello")

		newtime := time.Now()
		So(cba.Atime(), ShouldHappenBetween, oldtime, newtime)
		So(cba.Mtime(), ShouldHappenBetween, oldtime, newtime)

		time.Sleep(1 * time.Millisecond)

		So(cba.Get(), ShouldEqual, "Hello")
		So(cba.Atime(), ShouldHappenBetween, newtime, time.Now())
		So(cba.Mtime(), ShouldHappenBetween, oldtime, newtime)
	})
}

func TestMuxKeyExec(t *testing.T) {
	defer leaktest.Check(t)()

	st := Settings[string]{}
	st.Timeout = 2 * time.Millisecond

	var state = gobreaker.StateClosed
	st.OnStateChange = func(name string, from gobreaker.State, to gobreaker.State) {
		state = to
	}

	st.ExecClosure = func(input string) func() (string, error) {
		return func() (string, error) {
			if input == "yes" {
				return "yes", nil
			}
			return "no", fmt.Errorf("Noo")
		}
	}

	cbm := NewMux(st)
	defer cbm.Close()

	Convey("When a new mux is created, and there are no problems, the 'breaker should stay closed.", t, func() {

		for range 20 {
			v, e := cbm.GetKeyExec("yes breaker", "yes")
			So(state, ShouldEqual, gobreaker.StateClosed)
			So(v, ShouldEqual, "yes")
			So(e, ShouldBeNil)
		}

		Convey("... but when a problem does occur, the 'breaker should fly open after five fails.", func() {
			defer cbm.Delete("no")

			for i := range 20 {
				_, e := cbm.GetKeyExec("no breaker", "no")
				So(e, ShouldNotBeNil)
				if i < 5 {
					So(state, ShouldEqual, gobreaker.StateClosed)
				} else {
					// >= 5
					So(state, ShouldEqual, gobreaker.StateOpen)
				}
			}

		})
	})
}

func TestMuxSimple(t *testing.T) {
	defer leaktest.Check(t)()

	st := Settings[string]{}
	st.Timeout = 2 * time.Millisecond

	var state = gobreaker.StateClosed
	st.OnStateChange = func(name string, from gobreaker.State, to gobreaker.State) {
		state = to
	}

	st.ExecClosure = func(input string) func() (string, error) {
		return func() (string, error) {
			if input == "yes" {
				return "yes", nil
			}
			return "no", fmt.Errorf("Noo")
		}
	}

	cbm := NewMux(st)
	defer cbm.Close()

	Convey("When a new mux is created, and there are no problems, the 'breaker should stay closed.", t, func() {

		for range 20 {
			v, e := cbm.Get("yes")
			So(state, ShouldEqual, gobreaker.StateClosed)
			So(v, ShouldEqual, "yes")
			So(e, ShouldBeNil)
		}

		Convey("... but when a problem does occur, the 'breaker should fly open after five fails.", func() {
			defer cbm.Delete("no")

			for i := range 20 {
				_, e := cbm.Get("no")
				So(e, ShouldNotBeNil)
				if i < 5 {
					So(state, ShouldEqual, gobreaker.StateClosed)
				} else {
					// >= 5
					So(state, ShouldEqual, gobreaker.StateOpen)
				}
			}

		})
	})
}

func TestMuxSimpleBytes(t *testing.T) {
	defer leaktest.Check(t)()

	st := Settings[[]byte]{}
	st.Timeout = 2 * time.Millisecond

	var state = gobreaker.StateClosed
	st.OnStateChange = func(name string, from gobreaker.State, to gobreaker.State) {
		state = to
	}

	st.ExecClosure = func(input string) func() ([]byte, error) {
		return func() ([]byte, error) {
			if input == "yes" {
				return []byte("yes"), nil
			}
			return []byte("no"), fmt.Errorf("Noo")
		}
	}

	cbm := NewMux(st)
	defer cbm.Close()

	Convey("When a new mux is created, and there are no problems, the 'breaker should stay closed.", t, func() {

		for range 20 {
			v, e := cbm.Get("yes")
			So(state, ShouldEqual, gobreaker.StateClosed)
			So(v, ShouldEqual, []byte("yes"))
			So(e, ShouldBeNil)
		}

		Convey("... but when a problem does occur, the 'breaker should fly open after five fails.", func() {
			defer cbm.Delete("no")

			for i := range 20 {
				_, e := cbm.Get("no")
				So(e, ShouldNotBeNil)
				if i < 5 {
					So(state, ShouldEqual, gobreaker.StateClosed)
				} else {
					// >= 5
					So(state, ShouldEqual, gobreaker.StateOpen)
				}
			}

		})
	})
}

func TestMuxExpireAfter(t *testing.T) {
	defer leaktest.Check(t)()

	st := Settings[string]{}
	st.Timeout = 2 * time.Millisecond
	st.ExpireAfter = 50 * time.Millisecond
	st.ExpireCheck = 10 * time.Millisecond

	st.ExecClosure = func(input string) func() (string, error) {
		return func() (string, error) {
			if input == "yes" {
				return "yes", nil
			}
			return "no", fmt.Errorf("Noo")
		}
	}

	cbm := NewMux(st)
	defer cbm.Close()

	Convey("When a mux is created, configured with ExpireCheck and ExpireAfter is set, and a bunch of breakers are added, and after waiting, an old 'breaker has expired and is gone, but a fresher one is still around.", t, func() {
		cbm.Get("no")
		cbm.Get("yes")
		_, nok := cbm.breakers.Load("no")
		_, yok := cbm.breakers.Load("yes")
		So(yok, ShouldBeTrue)
		So(nok, ShouldBeTrue)

		cbm.Get("no")
		<-time.After(30 * time.Millisecond)
		cbm.Get("no")
		<-time.After(30 * time.Millisecond)
		_, nok = cbm.breakers.Load("no")
		_, yok = cbm.breakers.Load("yes")
		So(yok, ShouldBeFalse)
		So(nok, ShouldBeTrue)

	})
}

func TestMuxExpireClear(t *testing.T) {
	defer leaktest.Check(t)()

	st := Settings[string]{}
	st.Timeout = 2 * time.Millisecond
	st.ExpireCheck = 10 * time.Millisecond

	st.ExecClosure = func(input string) func() (string, error) {
		return func() (string, error) {
			if input == "yes" {
				return "yes", nil
			}
			return "no", fmt.Errorf("Noo")
		}
	}

	cbm := NewMux(st)
	defer cbm.Close()

	Convey("When a mux is created, configured with ExpireCheck but no ExpireAfter, and a bunch of breakers are added, and after waiting, all of the breakers are gone.", t, func() {
		cbm.Get("no")
		cbm.Get("yes")
		_, nok := cbm.breakers.Load("no")
		_, yok := cbm.breakers.Load("yes")
		So(yok, ShouldBeTrue)
		So(nok, ShouldBeTrue)

		cbm.Get("no")
		<-time.After(30 * time.Millisecond)
		cbm.Get("no")
		<-time.After(30 * time.Millisecond)
		_, nok = cbm.breakers.Load("no")
		_, yok = cbm.breakers.Load("yes")
		So(yok, ShouldBeFalse)
		So(nok, ShouldBeFalse)

	})
}

// Benchmark_HttpGet loops a function that is like an ExecFunc, that http.Get's a URL and returns the read body or an error.
func Benchmark_HttpGet(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	f := func(url string) ([]byte, error) {
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		return body, nil
	}

	var err error
	b.ResetTimer()
	for b.Loop() {
		_, err = f(ts.URL)
		if err != nil {
			panic(err)
		}
	}

}

// Benchmark_Gobreaker loops a function pulling through gobreaker, that http.Get's a URL and returns the read body or an error.
func Benchmark_Gobreaker(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	var cb *gobreaker.CircuitBreaker[[]byte]
	var st gobreaker.Settings
	st.OnStateChange = func(name string, from gobreaker.State, to gobreaker.State) {
		panic(fmt.Errorf("%s: %s -> %s", name, from, to))
	}
	cb = gobreaker.NewCircuitBreaker[[]byte](st)

	f := func(url string) ([]byte, error) {
		body, err := cb.Execute(func() ([]byte, error) {
			resp, err := http.Get(url)
			if err != nil {
				return nil, err
			}

			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}

			return body, nil
		})
		if err != nil {
			return nil, err
		}

		return body, nil
	}

	var err error
	b.ResetTimer()
	for b.Loop() {
		_, err = f(ts.URL)
		if err != nil {
			panic(err)
		}
	}

}

// Benchmark_Mux loops an ExecFunc closure, that http.Get's a URL and returns the read body or an error.
func Benchmark_Mux(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	var st = Settings[[]byte]{}
	st.OnStateChange = func(name string, from gobreaker.State, to gobreaker.State) {
		panic(fmt.Errorf("%s: %s -> %s", name, from, to))
	}

	st.ExecClosure = func(url string) func() ([]byte, error) {
		return func() ([]byte, error) {
			resp, err := http.Get(url)
			if err != nil {
				return nil, err
			}

			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}

			return body, nil
		}
	}

	cbm := NewMux(st)
	defer cbm.Close()

	var err error
	b.ResetTimer()
	for b.Loop() {
		_, err = cbm.Get(ts.URL)
		if err != nil {
			panic(err)
		}
	}

}

// Benchmark_MuxFail loops an ExecFunc closure, that fails to http.Get's a URL and returns an error.
func Benchmark_MuxFail(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, client")
	}))
	defer ts.Close()

	var st = Settings[[]byte]{}

	st.ExecClosure = func(url string) func() ([]byte, error) {
		return func() ([]byte, error) {
			resp, err := http.Get(url)
			if err != nil {
				return nil, err
			}

			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}

			return body, nil
		}
	}

	cbm := NewMux(st)
	defer cbm.Close()

	// Close the ts
	ts.Close()

	var err error
	b.ResetTimer()
	for b.Loop() {
		_, err = cbm.Get(ts.URL)
		if err != nil {
			// We fail, deliberately
		}
	}

}
