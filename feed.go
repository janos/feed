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
	subscriptions map[T][]*subscription[T, M]
	mu            sync.RWMutex

	wg       sync.WaitGroup
	quit     chan struct{}
	quitOnce sync.Once
}

// NewFeed constructs new Feed with topic type T and message type M.
func NewFeed[T comparable, M any]() *Feed[T, M] {
	return &Feed[T, M]{
		subscriptions: make(map[T][]*subscription[T, M]),
		quit:          make(chan struct{}),
	}
}

// Subscribe returns a channel from which messages M, that are sent to the Feed
// on the same topic, can be read from. Message delivery preserves ordering and
// is guaranteed, so the channel should be read to avoid keeping unread messages
// in memory. After cancel function call, all resources ang goroutines are
// released even if not all messages are read from channel.
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

	s := newSubscription(f, channel)

	f.subscriptions[topic] = append(f.subscriptions[topic], s)

	return channel, func() { f.unsubscribe(topic, s) }
}

func (f *Feed[T, M]) unsubscribe(topic T, s *subscription[T, M]) {
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
		s.send(message)

		n++
	}

	return n
}

type subscription[T comparable, M any] struct {
	feed *Feed[T, M]

	channel chan M

	buf []M
	mu  sync.Mutex

	quit chan struct{}
	wg   sync.WaitGroup
}

func newSubscription[T comparable, M any](feed *Feed[T, M], channel chan M) *subscription[T, M] {
	return &subscription[T, M]{
		feed:    feed,
		channel: channel,
		buf:     make([]M, 0),
		quit:    make(chan struct{}),
	}
}

func (s *subscription[T, M]) send(message M) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.buf) > 0 {
		s.buf = append(s.buf, message)
		return
	}

	select {
	case s.channel <- message:
	case <-s.quit:
		return
	case <-s.feed.quit:
		return
	default:
		s.buf = append(s.buf, message)

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()

			for {
				s.mu.Lock()
				message := s.buf[0]
				s.mu.Unlock()

				select {
				case s.channel <- message:
				case <-s.quit:
					return
				case <-s.feed.quit:
					return
				}

				s.mu.Lock()
				s.buf = s.buf[1:]
				if len(s.buf) == 0 {
					s.mu.Unlock()
					return
				}
				s.mu.Unlock()
			}

		}()
	}
}

func (s *subscription[T, M]) close() {
	close(s.quit)
	s.wg.Wait()
	close(s.channel)
}
