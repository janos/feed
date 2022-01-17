// Copyright (c) 2022, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed

import (
	"sync"
)

// Trigger notifies over topic based subscription channels if one or more
// trigger events occurred. Trigger is useful in situations where only
// information that something happened is useful, not information about all
// occurred events.
type Trigger[T comparable] struct {
	channels map[T][]chan struct{}
	mu       sync.RWMutex
}

// NewTrigger constructs a new Trigger instance.
func NewTrigger[T comparable]() *Trigger[T] {
	return &Trigger[T]{
		channels: make(map[T][]chan struct{}),
	}
}

// Subscribe returns a channel of empty structs which will return if at least
// one Trigger call has been done on the same topic.
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

// Trigger notifies all subscritions on the provided topic. Notifications will
// be delivered to subscribers when each of them is ready to receive it, without
// blocking this method call. Returned integer is a number of subscriptions that
// will be notified.
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
