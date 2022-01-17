// Copyright (c) 2022, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed

import (
	"sync"
)

type Trigger[T comparable] struct {
	channels map[T][]chan struct{}
	mu       sync.RWMutex
}

func NewTrigger[T comparable]() *Trigger[T] {
	return &Trigger[T]{
		channels: make(map[T][]chan struct{}),
	}
}

func (t *Trigger[T]) Subscribe(topic T) (c <-chan struct{}, cancel func()) {
	channel := make(chan struct{}, 1)

	t.mu.Lock()
	defer t.mu.Unlock()

	t.channels[topic] = append(t.channels[topic], channel)

	return channel, func() { t.unsubscribe(topic, channel) }
}

func (t *Trigger[T]) unsubscribe(topic T, c <-chan struct{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, ch := range t.channels[topic] {
		if ch == c {
			t.channels[topic] = append(t.channels[topic][:i], t.channels[topic][i+1:]...)
			close(ch)
		}
	}
}

func (t *Trigger[T]) Trigger(topic T) (n int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, c := range t.channels[topic] {
		select {
		case c <- struct{}{}:
		default:
		}

		n++
	}

	return n
}
