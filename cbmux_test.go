package breakermux

import (
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/sony/gobreaker/v2"
)

func Example() {

	// Set the timeout to 2ms, and print something nice when the state changes
	var st = DefaultSettings
	st.Timeout = 2 * time.Millisecond
	st.OnStateChange = func(name string, from gobreaker.State, to gobreaker.State) {
		fmt.Printf("%s: %+v -> %+v\n", name, from, to)
	}

	// Our ExecFunc sleeps for a half-second so it is visually obvious when it is being
	// called ('breaker closed), versus skipped ('breaker open)
	var efunc = func(input string) func() (string, error) {
		return func() (string, error) {
			time.Sleep(500 * time.Millisecond)
			if input == "yes" {
				return "yes", nil
			}
			return "no", fmt.Errorf("Noo")
		}
	}

	// Create a mux, passing it our Settings and ExecFunc
	cbm := NewMux(st, efunc)

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

func TestMuxSimple(t *testing.T) {

	st := DefaultSettings
	st.Timeout = 2 * time.Millisecond

	var state = gobreaker.StateClosed
	st.OnStateChange = func(name string, from gobreaker.State, to gobreaker.State) {
		state = to
	}

	var efunc = func(input string) func() (string, error) {
		return func() (string, error) {
			if input == "yes" {
				return "yes", nil
			}
			return "no", fmt.Errorf("Noo")
		}
	}

	cbm := NewMux(st, efunc)
	defer cbm.Clear()

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
