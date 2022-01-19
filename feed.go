// Copyright (c) 2022, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed

import (
	"sync"
)

// Feed defines a set of subscriptions per topic T which receive messages sent
// to the Feed.
type Feed[T comparable, M any] struct {
	subscriptions map[T][]*subscription[M]
	mu            sync.RWMutex

	wg       sync.WaitGroup
	quit     chan struct{}
	quitOnce sync.Once
}

// NewFeed constructs new Feed with topic type T and message type M.
func NewFeed[T comparable, M any]() *Feed[T, M] {
	return &Feed[T, M]{
		subscriptions: make(map[T][]*subscription[M]),
		quit:          make(chan struct{}),
	}
}

// Subscribe returns a channel from which messages M, that are sent to the Feed
// on the same topic, can be read from. Message delivery is guaranteed, so the
// channel should be read to avoid possible high number of goroutines. After
// cancel function call, all resources ang goroutines are released even if not
// all messages are read from channel.
func (f *Feed[T, M]) Subscribe(topic T) (c <-chan M, cancel func()) {
	channel := make(chan M, 1) // buffer 1 not to block on Send method

	select {
	case <-f.quit:
		close(channel)
		return channel, func() {}
	default:
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	s := newSubscription(channel)

	f.subscriptions[topic] = append(f.subscriptions[topic], s)

	return channel, func() { f.unsubscribe(topic, s) }
}

func (f *Feed[T, M]) unsubscribe(topic T, s *subscription[M]) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, sub := range f.subscriptions[topic] {
		if sub == s {
			f.subscriptions[topic] = append(f.subscriptions[topic][:i], f.subscriptions[topic][i+1:]...)
			s.close()
		}
	}
}

// Close terminates all subscriptions and releases acquired resources.
func (f *Feed[T, M]) Close() error {
	f.quitOnce.Do(func() {
		close(f.quit)
	})
	f.wg.Wait()

	f.mu.Lock()
	defer f.mu.Unlock()

	for topic, subscriptions := range f.subscriptions {
		for _, s := range subscriptions {
			s.close()
		}
		f.subscriptions[topic] = nil
	}

	return nil
}

// Send sends a message to all sunscribed channels to topic. Messages will be
// delivered to subscribers when each of them is ready to receive it, without
// blocking this method call. The returned integer is the number of subscribers
// that should receive the message.
func (f *Feed[T, M]) Send(topic T, message M) (n int) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, s := range f.subscriptions[topic] {
		// try to send message to the channel
		select {
		case s.channel <- message:
		case <-s.quit:
			return
		case <-f.quit:
			return
		default:
			// if channel is blocked,
			// wait in goroutine to send the message
			s := s

			f.wg.Add(1)
			go func() {
				defer f.wg.Done()

				select {
				case s.channel <- message:
				case <-s.quit:
					return
				case <-f.quit:
					return
				}
			}()
		}

		n++
	}

	return n
}

type subscription[M any] struct {
	channel  chan M
	quitOnce sync.Once
	quit     chan struct{}
	wg       sync.WaitGroup
}

func newSubscription[M any](channel chan M) *subscription[M] {
	return &subscription[M]{
		channel: channel,
		quit:    make(chan struct{}),
	}
}

func (s *subscription[M]) close() {
	s.quitOnce.Do(func() {
		close(s.quit)
	})
	s.wg.Wait()
}
