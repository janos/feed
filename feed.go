// Copyright (c) 2022, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed

import (
	"context"
	"sync"
)

type Feed[T comparable, M any] struct {
	channels map[T][]chan M
	mu       sync.RWMutex

	wg       sync.WaitGroup
	quit     chan struct{}
	quitOnce sync.Once
}

func New[T comparable, M any]() (f *Feed[T, M]) {
	return &Feed[T, M]{
		channels: make(map[T][]chan M),
		quit:     make(chan struct{}),
	}
}

func (f *Feed[T, M]) Subscribe(topic T) (c <-chan M, stop func()) {
	select {
	case <-f.quit:
		return nil, nil
	default:
	}

	channel := make(chan M)

	f.mu.Lock()
	defer f.mu.Unlock()

	f.channels[topic] = append(f.channels[topic], channel)

	return channel, func() { f.unsubscribe(topic, channel) }
}

func (f *Feed[T, M]) unsubscribe(topic T, c <-chan M) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i, ch := range f.channels[topic] {
		if ch == c {
			f.channels[topic] = append(f.channels[topic][:i], f.channels[topic][i+1:]...)
			close(ch)
		}
	}
}

func (f *Feed[T, M]) Shutdown(ctx context.Context) error {
	f.quitOnce.Do(func() {
		close(f.quit)
	})
	done := make(chan struct{})
	go func() {
		f.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for topic, channels := range f.channels {
		for _, c := range channels {
			close(c)
		}
		f.channels[topic] = nil
	}

	return nil
}

func (f *Feed[T, M]) Send(topic T, message M) (n int) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for _, c := range f.channels[topic] {
		c := c

		go func() {
			select {
			case c <- message:
			case <-f.quit:
				return
			}
		}()

		n++
	}

	return n
}