// Copyright (c) 2022, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed_test

import (
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"time"

	"resenje.org/feed"
)

func TestFeed_singleMessage(t *testing.T) {
	f := feed.NewFeed[string, int]()
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})

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
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})

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
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})

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
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})

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
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})

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
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})

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

func TestFeed_ordering(t *testing.T) {
	f := feed.NewFeed[string, int]()
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})

	got := make([]int, 0)

	s, cancel := f.Subscribe("topic1")
	defer cancel()

	messages := make([]int, 0)

	count := 1000

	for i := 0; i < count; i++ {
		messages = append(messages, i)
		n := f.Send("topic1", i)
		assert(t, "", n, 1)
	}

	for i := 0; i < count; i++ {
		got = append(got, <-s)
	}

	assert(t, "", got, messages)
}

func TestFeed_stressTest(t *testing.T) {
	f := feed.NewFeed[string, int]()
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})

	count := 1000
	lastReceived := -1

	s, cancel := f.Subscribe("topic1")
	defer cancel()

	random1 := rand.New(rand.NewSource(time.Now().UnixNano()))
	random2 := rand.New(rand.NewSource(time.Now().UnixNano()))

	receiveDone := make(chan struct{})
	go func() {
		defer close(receiveDone)
		for {
			m, ok := <-s
			if !ok {
				return
			}
			if m != lastReceived+1 {
				assert(t, "", m, lastReceived+1)
			}
			lastReceived = m
			time.Sleep(time.Duration(random1.Int63n(1000)) * time.Microsecond)
		}
	}()

	sendDone := make(chan struct{})
	go func() {
		defer close(sendDone)
		for i := 0; i < count; i++ {
			n := f.Send("topic1", i)
			assert(t, "", n, 1)
			time.Sleep(time.Duration(random2.Int63n(1000)) * time.Microsecond)
		}
	}()

	<-sendDone

	cancel()

	<-receiveDone
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
