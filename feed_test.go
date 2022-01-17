// Copyright (c) 2022, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed_test

import (
	"reflect"
	"sync"
	"testing"

	"resenje.org/feed"
)

func TestFeed_singleMessage(t *testing.T) {
	f := feed.NewFeed[string, int]()
	defer f.Close()

	got := make([]int, 0)

	c := newCond()

	s, cancel := f.Subscribe("topic1")
	defer cancel()

	go func() {
		for m := range s {
			signalCond(c, func() {
				got = append(got, m)
			})
		}
	}()

	waitCond(c, func() {
		n := f.Send("topic1", 25)
		assert(t, "", n, 1)
	})

	assert(t, "", got, []int{25})
}

func TestFeed_twoMessages(t *testing.T) {
	f := feed.NewFeed[string, int]()
	defer f.Close()

	got := make([]int, 0)

	c := newCond()

	s, cancel := f.Subscribe("topic1")
	defer cancel()

	go func() {
		for m := range s {
			signalCond(c, func() {
				got = append(got, m)
			})
		}
	}()

	waitCond(c, func() {
		n := f.Send("topic1", 25)
		assert(t, "", n, 1)
	})

	assert(t, "", got, []int{25})

	waitCond(c, func() {
		n := f.Send("topic1", 42)
		assert(t, "", n, 1)
	})

	assert(t, "", got, []int{25, 42})
}

func TestFeed_multipleSubscriptions(t *testing.T) {
	f := feed.NewFeed[string, int]()
	defer f.Close()

	got1 := make([]int, 0)
	c1 := newCond()

	s1, cancel1 := f.Subscribe("topic1")
	defer cancel1()

	go func() {
		for m := range s1 {
			signalCond(c1, func() {
				got1 = append(got1, m)
			})
		}
	}()

	got2 := make([]int, 0)
	c2 := newCond()

	s2, cancel2 := f.Subscribe("topic1")
	defer cancel2()

	go func() {
		for m := range s2 {
			signalCond(c2, func() {
				got2 = append(got2, m)
			})
		}
	}()

	waitCond(c1, func() {
		waitCond(c2, func() {
			n := f.Send("topic1", 25)
			assert(t, "", n, 2)
		})
	})

	assert(t, "", got1, []int{25})
	assert(t, "", got2, []int{25})

	cancel2()

	got3 := make([]int, 0)
	c3 := newCond()

	s3, cancel3 := f.Subscribe("topic1")
	defer cancel3()

	go func() {
		for m := range s3 {
			signalCond(c3, func() {
				got3 = append(got3, m)
			})
		}
	}()

	waitCond(c1, func() {
		waitCond(c3, func() {
			n := f.Send("topic1", 42)
			assert(t, "", n, 2)
		})
	})

	assert(t, "", got1, []int{25, 42})
	assert(t, "", got2, []int{25})
	assert(t, "", got3, []int{42})
}

func TestFeed_multipleTopics(t *testing.T) {
	f := feed.NewFeed[string, int]()
	defer f.Close()

	got1 := make([]int, 0)
	c1 := newCond()

	s1, cancel1 := f.Subscribe("topic1")
	defer cancel1()

	go func() {
		for m := range s1 {
			signalCond(c1, func() {
				got1 = append(got1, m)
			})
		}
	}()

	got2 := make([]int, 0)
	c2 := newCond()

	s2, cancel2 := f.Subscribe("topic2")
	defer cancel2()

	go func() {
		for m := range s2 {
			signalCond(c2, func() {
				got2 = append(got2, m)
			})
		}
	}()

	waitCond(c1, func() {
		n := f.Send("topic1", 25)
		assert(t, "", n, 1)
	})

	assert(t, "", got1, []int{25})
	assert(t, "", got2, []int{})

	waitCond(c2, func() {
		n := f.Send("topic2", 42)
		assert(t, "", n, 1)
	})

	assert(t, "", got1, []int{25})
	assert(t, "", got2, []int{42})
}

func TestFeed_shutdown(t *testing.T) {
	f := feed.NewFeed[string, int]()
	err := f.Close()
	assert(t, "", err, nil)

	c, cancel := f.Subscribe("topic")

	m, ok := <-c
	assert(t, "", m, 0)
	assert(t, "", ok, false)

	if cancel == nil {
		t.Error("cancel function iz nil")
	}

	n := f.Send("topic", 25)
	assert(t, "", n, 0)
}

func TestFeed_shutdownWithUnreadMessages(t *testing.T) {
	f := feed.NewFeed[string, int]()
	defer f.Close()

	c, _ := f.Subscribe("topic")

	n := f.Send("topic", 25)
	assert(t, "", n, 1)

	n = f.Send("topic", 42)
	assert(t, "", n, 1)

	n = f.Send("topic", 100)
	assert(t, "", n, 1)

	n = f.Send("topic", 200)
	assert(t, "", n, 1)

	m, ok := <-c
	assert(t, "", m, 25)
	assert(t, "", ok, true)

	err := f.Close()
	assert(t, "", err, nil)
}

func TestFeed_cancelWithUnreadMessages(t *testing.T) {
	f := feed.NewFeed[string, int]()
	defer f.Close()

	c, cancel := f.Subscribe("topic")

	n := f.Send("topic", 25)
	assert(t, "", n, 1)

	n = f.Send("topic", 42)
	assert(t, "", n, 1)

	n = f.Send("topic", 100)
	assert(t, "", n, 1)

	n = f.Send("topic", 200)
	assert(t, "", n, 1)

	m, ok := <-c
	assert(t, "", m, 25)
	assert(t, "", ok, true)

	cancel()
}

func assert[T any](t testing.TB, message string, got, want T) {
	t.Helper()

	if message != "" {
		message = message + ": "
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("%sgot %v, want %v", message, got, want)
	}
}

func newCond() *sync.Cond {
	return sync.NewCond(new(sync.Mutex))
}

func signalCond(c *sync.Cond, f func()) {
	c.L.Lock()
	defer c.L.Unlock()

	f()
	c.Signal()
}

func waitCond(c *sync.Cond, f func()) {
	c.L.Lock()
	defer c.L.Unlock()

	f()
	c.Wait()
}
