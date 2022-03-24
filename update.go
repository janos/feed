// Copyright (c) 2022, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed

import (
	"sync"
)

// Update defines a set of subscriptions per topic T which receive messages sent
// to the Update.
type Update[T comparable, M any] struct {
	subscriptions map[T][]*updateSubscription[T, M]
	mu            sync.RWMutex

	wg       sync.WaitGroup
	quit     chan struct{}
	quitOnce sync.Once
}

// NewFeed constructs new Feed with topic type T and message type M.
func NewUpdate[T comparable, M any]() *Update[T, M] {
	return &Update[T, M]{
		subscriptions: make(map[T][]*updateSubscription[T, M]),
		quit:          make(chan struct{}),
	}
}

// Subscribe returns a channel from which messages M, that are sent to the Feed
// on the same topic, can be read from. Message delivery preserves ordering and
// is guaranteed, so the channel should be read to avoid keeping unread messages
// in memory. After cancel function call, all resources ang goroutines are
// released even if not all messages are read from channel.
func (u *Update[T, M]) Subscribe(topic T) (c <-chan M, cancel func()) {
	channel := make(chan M)

	select {
	case <-u.quit:
		close(channel)
		return channel, func() {}
	default:
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	s := newUpdateSubscription(u, channel)

	u.subscriptions[topic] = append(u.subscriptions[topic], s)

	return channel, func() { u.unsubscribe(topic, s) }
}

func (u *Update[T, M]) unsubscribe(topic T, s *updateSubscription[T, M]) {
	u.mu.Lock()
	defer u.mu.Unlock()

	for i, sub := range u.subscriptions[topic] {
		if sub == s {
			u.subscriptions[topic] = append(u.subscriptions[topic][:i], u.subscriptions[topic][i+1:]...)
			s.close()
		}
	}
}

// Close terminates all subscriptions and releases acquired resources.
func (u *Update[T, M]) Close() error {
	u.quitOnce.Do(func() {
		close(u.quit)
	})
	u.wg.Wait()

	u.mu.Lock()
	defer u.mu.Unlock()

	for topic, subscriptions := range u.subscriptions {
		for _, s := range subscriptions {
			s.close()
		}
		u.subscriptions[topic] = nil
	}

	return nil
}

// Send sends a message to all sunscribed channels to topic. Messages will be
// delivered to subscribers when each of them is ready to receive it, without
// blocking this method call. The returned integer is the number of subscribers
// that should receive the message.
func (u *Update[T, M]) Send(topic T, message M) (n int) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	for _, s := range u.subscriptions[topic] {
		s.send(message)

		n++
	}

	return n
}

type updateSubscription[T comparable, M any] struct {
	feed *Update[T, M]

	channel chan M
	update  chan M
	updated chan struct{}

	quit chan struct{}
	wg   sync.WaitGroup
}

func newUpdateSubscription[T comparable, M any](u *Update[T, M], channel chan M) *updateSubscription[T, M] {
	return &updateSubscription[T, M]{
		feed:    u,
		channel: channel,
		update:  make(chan M),
		updated: make(chan struct{}),
		quit:    make(chan struct{}),
	}
}

func (s *updateSubscription[T, M]) send(message M) {
	select {
	case s.channel <- message:

	case s.update <- message:
		select {
		case <-s.updated:
		case <-s.quit:
		case <-s.feed.quit:
		}
		return

	case <-s.quit:
		return

	case <-s.feed.quit:
		return

	default:

		ready := make(chan struct{})
		done := make(chan struct{})

		channel := s.channel

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer close(done)

			for {
				select {
				case channel <- message:
					return

				case message = <-s.update:
					channel = nil

				case ready <- struct{}{}:

				case s.updated <- struct{}{}:
					channel = s.channel

				case <-s.quit:
					return

				case <-s.feed.quit:
					return

				}
			}
		}()

		select {
		case <-ready:
		case <-done:
		case <-s.quit:
		case <-s.feed.quit:
		}
	}
}

func (s *updateSubscription[T, M]) close() {
	close(s.quit)
	s.wg.Wait()
	close(s.channel)
}
