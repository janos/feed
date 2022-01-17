// Copyright (c) 2022, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed_test

import (
	"testing"

	"resenje.org/feed"
)

func TestTrigger_triggerOnce(t *testing.T) {
	tr := feed.NewTrigger[int]()

	s, cancel := tr.Subscribe(1)
	defer cancel()

	c := newCond()

	var got bool

	go func() {
		for range s {
			signalCond(c, func() {
				got = true
			})
		}
	}()

	waitCond(c, func() {
		n := tr.Trigger(1)
		assert(t, "", n, 1)
	})

	assert(t, "", got, true)
}

func TestTrigger_triggerMultiple(t *testing.T) {
	tr := feed.NewTrigger[int]()

	s, cancel := tr.Subscribe(1)
	defer cancel()

	c := newCond()

	var gotCount int

	stopRead := make(chan struct{})

	go func() {
		for {
			select {
			case <-s:
				signalCond(c, func() {
					gotCount++
				})
			case <-stopRead:
				return
			}
		}
	}()

	waitCond(c, func() {
		n := tr.Trigger(1)
		assert(t, "", n, 1)
	})

	assert(t, "", gotCount, 1)

	waitCond(c, func() {
		n := tr.Trigger(1)
		assert(t, "", n, 1)
	})

	assert(t, "", gotCount, 2)

	close(stopRead)

	for i := 0; i < 10; i++ {
		n := tr.Trigger(1)
		assert(t, "", n, 1)
	}

	read := make(chan struct{})

	go func() {
		for range s {
			gotCount++
			close(read)
		}
	}()

	<-read

	assert(t, "", gotCount, 3)
}

func TestTrigger_multipleTopics(t *testing.T) {
	tr := feed.NewTrigger[int]()

	s1, cancel1 := tr.Subscribe(1)
	defer cancel1()

	c1 := newCond()

	var got1 bool

	go func() {
		for range s1 {
			signalCond(c1, func() {
				got1 = true
			})
		}
	}()

	s2, cancel2 := tr.Subscribe(1)
	defer cancel2()

	c2 := newCond()

	var got2 bool

	go func() {
		for range s2 {
			signalCond(c2, func() {
				got2 = true
			})
		}
	}()

	waitCond(c1, func() {
		waitCond(c2, func() {
			n := tr.Trigger(1)
			assert(t, "", n, 2)
		})
	})

	assert(t, "", got1, true)
	assert(t, "", got2, true)
}
