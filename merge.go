// Copyright (c) 2025, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed

import (
	"context"
	"sync"
)

// Merge combines multiple input channels of type T into a single output channel.
// It uses a goroutine for each input channel to forward values.
// The merging process stops if the provided context is cancelled.
// The output channel is closed once all input channels are closed and drained,
// or if the context is cancelled.
// Nil input channels are ignored.
// It can be used to combine multiple feed or trigger subscriptions.
func Merge[T any](ctx context.Context, chans ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup

	// output is a helper function run in a goroutine for each channel.
	// It reads values from a single input channel and sends them to the out channel.
	// It stops if the context is cancelled or the input channel is closed.
	output := func(c <-chan T) {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done(): // Check if context has been cancelled.
				return // Exit goroutine if context is cancelled.
			case val, ok := <-c:
				if !ok { // Input channel c is closed.
					return // Exit goroutine.
				}
				// Try to send the value to out, but also listen for context cancellation.
				select {
				case out <- val:
				case <-ctx.Done(): // Context cancelled while trying to send.
					return // Exit goroutine.
				}
			}
		}
	}

	// Start a goroutine for each non-nil input channel.
	for _, c := range chans {
		if c != nil { // Ignore nil channels
			wg.Add(1)
			go output(c) // Pass c to the goroutine to capture its current value
		}
	}

	// Start a goroutine to wait for all other goroutines to complete,
	// or for the context to be cancelled, then close the out channel.
	go func() {
		// Wait for all goroutines to finish or context to be cancelled.
		// It's possible wg.Wait() finishes first, or ctx.Done() is selected first.
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// All goroutines completed normally.
		case <-ctx.Done():
			// Context was cancelled. Child goroutines will also see this and exit.
			// We still wait for them to clean up to avoid potential partial writes
			// if they were in the middle of sending to 'out'.
			// However, since child goroutines also select on ctx.Done(),
			// they should terminate quickly.
		}
		close(out) // Close the output channel.
	}()

	return out
}
