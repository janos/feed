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
	// then close the out channel. This goroutine also respects context cancellation.
	go func() {
		// allOutputGoroutinesDone channel signals that wg.Wait() has completed.
		allOutputGoroutinesDone := make(chan struct{})
		go func() {
			wg.Wait()                      // Wait for all output goroutines to call wg.Done()
			close(allOutputGoroutinesDone) // Signal completion
		}()

		select {
		case <-allOutputGoroutinesDone:
			// All output goroutines completed their work (e.g., input channels drained).
			// It's safe to close 'out'.
		case <-ctx.Done():
			// Context was cancelled.
			// The output goroutines are designed to detect this cancellation
			// and will call wg.Done() upon their termination.
			// We must wait for all of them to actually finish (i.e., for wg.Wait() to unblock)
			// before we can safely close the 'out' channel.
			<-allOutputGoroutinesDone // Wait for wg.Wait() to complete.
		}
		close(out) // Close the output channel.
	}()

	return out
}
