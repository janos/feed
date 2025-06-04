// Copyright (c) 2022, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed_test

import (
	"math/rand"
	"testing"
	"time"

	"resenje.org/feed"
)

func TestUpdate_singleMessage(t *testing.T) {
	f := feed.NewUpdate[string, int]()
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

func TestUpdate_twoMessages(t *testing.T) {
	f := feed.NewUpdate[string, int]()
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

func TestUpdate_multipleSubscriptions(t *testing.T) {
	f := feed.NewUpdate[string, int]()
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

func TestUpdate_multipleTopics(t *testing.T) {
	f := feed.NewUpdate[string, int]()
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

func TestUpdate_shutdown(t *testing.T) {
	f := feed.NewUpdate[string, int]()
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

func TestUpdate_shutdownWithUnreadMessages(t *testing.T) {
	f := feed.NewUpdate[string, int]()
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
	assert(t, "", m, 200)
	assert(t, "", ok, true)

	err := f.Close()
	assert(t, "", err, nil)
}

func TestUpdate_cancelWithUnreadMessages(t *testing.T) {
	f := feed.NewUpdate[string, int]()
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
	assert(t, "", m, 200)
	assert(t, "", ok, true)

	cancel()
}

func TestUpdate_latestMessage(t *testing.T) {
	f := feed.NewUpdate[string, int]()
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})

	s, cancel := f.Subscribe("topic1")
	defer cancel()

	count := 100

	for i := 0; i < count; i++ {
		n := f.Send("topic1", i)
		assert(t, "", n, 1)
	}

	assert(t, "", <-s, count-1)
}

func TestUpdate_stressTest(t *testing.T) {
	f := feed.NewUpdate[string, int]()
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
			if m <= lastReceived {
				t.Errorf("got unexpected message %v, less than %v", m, lastReceived)
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
			time.Sleep(time.Duration(random2.Int63n(100)) * time.Microsecond)
		}
	}()

	<-sendDone

	cancel()

	<-receiveDone
}
