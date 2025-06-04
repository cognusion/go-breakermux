package breakermux

import (
	"fmt"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/sony/gobreaker/v2"
)

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
