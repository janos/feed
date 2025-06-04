// Copyright (c) 2025, Janoš Guljaš <janos@resenje.org>
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package feed_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"resenje.org/feed"
)

func TestMerge(t *testing.T) {
	var testCases = []struct {
		name   string
		inputs [][]int
	}{
		{"NoChannels", [][]int{}},
		{"OneEmptyChannel", [][]int{makeInput[int]()}},
		{"MultipleEmptyChannels", [][]int{makeInput[int](), makeInput[int](), makeInput[int]()}},
		{"OneChannelWithItems", [][]int{makeInput(1, 2, 3)}},
		{"MultipleChannelsWithItems", [][]int{makeInput(1, 2), makeInput(3, 4, 5), makeInput(6)}},
		{"ChannelsWithMixedEmptyAndItems", [][]int{makeInput(1), makeInput[int](), makeInput(2, 3)}},
		{"NilChannelsMixedIn", nil}, // Special case handled in TestMerge_WithNilChannels
	}

	for _, tc := range testCases {
		if tc.name == "NilChannelsMixedIn" { // Skip special case here
			continue
		}
		currentTC := tc
		runMergeTest(t, "Merge_"+currentTC.name, currentTC.inputs)
	}

	// Test case with nil channels specifically for Merge
	t.Run("Merge_NilChannelsMixedIn", func(t *testing.T) {
		ch1 := make(chan int, 1)
		ch1 <- 10
		close(ch1)

		var chNil <-chan int // nil channel

		ch2 := make(chan int, 1)
		ch2 <- 20
		close(ch2)

		inputs := []<-chan int{ch1, chNil, ch2, nil}
		expected := map[int]int{10: 1, 20: 1}
		totalExpected := 2

		merged := feed.Merge(context.Background(), inputs...)
		received := make(map[int]int)
		count := 0
		for val := range merged {
			received[val]++
			count++
		}
		if count != totalExpected {
			t.Errorf("expected %d, got %d", totalExpected, count)
		}
		if !reflect.DeepEqual(expected, received) {
			t.Errorf("expected %v, got %v", expected, received)
		}
	})
}

func TestMerge_Cancellation(t *testing.T) {
	t.Run("CancelImmediately", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		ch1 := make(chan int) // Unbuffered, will block
		// defer close(ch1) // Not strictly needed as it won't be written to

		merged := feed.Merge(ctx, ch1)

		select {
		case _, ok := <-merged:
			if ok {
				t.Error("expected merged channel to be closed or empty due to immediate cancellation, but received a value")
			}
			// If !ok, channel is closed as expected.
		case <-time.After(100 * time.Millisecond): // Shorter timeout for cancellation tests
			t.Error("merged channel did not close after immediate cancellation")
		}
	})

	t.Run("CancelAfterSomeItems", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())

		ch1 := make(chan int, 3)
		var item1, item2, item3 int // Zero values, actual value doesn't matter for this test structure
		ch1 <- item1
		ch1 <- item2
		ch1 <- item3
		// Don't close ch1 immediately, simulate ongoing source

		merged := feed.Merge(ctx, ch1)

		// Read one item to ensure merge started
		select {
		case _, ok := <-merged:
			if !ok {
				t.Fatal("merged channel closed prematurely")
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for first item")
		}

		cancel() // Cancel the context

		// Try to read remaining items, but expect it to close due to cancellation
		// It's possible some more items come through if they were already buffered
		// or sent before the cancellation signal was processed by all goroutines.
		// The key is that the channel eventually closes.
		for i := 0; i < 5; i++ { // Try to drain a few more, or detect closure
			select {
			case _, ok := <-merged:
				if !ok {
					return // Channel closed as expected after cancellation
				}
				// Received an item, this is okay if it was in flight
			case <-time.After(200 * time.Millisecond): // Increased timeout slightly
				t.Error("merged channel did not close after cancellation and draining potential in-flight items")
				return
			}
		}
		t.Error("merged channel did not close within reasonable attempts after cancellation")
	})

	t.Run("CancelWithBlockedSend", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())

		// ch1 will send one item, then block on the second if `out` isn't read.
		// `out` won't be read in this test after the first item, simulating a slow consumer.
		ch1 := make(chan int, 1) // Buffered by 1
		var item1, item2 int
		ch1 <- item1 // This will go into buffer
		// The goroutine for ch1 will block trying to send item2 if Merge's out channel is full or not read.

		go func() { // This goroutine will attempt to send the second item
			// It might block here if the merge output is not consumed
			// and the context is not cancelled quickly enough.
			// However, the select in the output goroutine should pick up ctx.Done()
			select {
			case ch1 <- item2:
			case <-time.After(500 * time.Millisecond): // Don't let this goroutine hang indefinitely
				// This case is mostly to prevent test hangs if logic is flawed.
				// The primary check is on the merged channel closing.
			}
			// close(ch1) // Not closing, to simulate an ongoing source that gets cancelled
		}()

		merged := feed.Merge(ctx, ch1)

		// Do not read from merged initially to let internal buffers potentially fill
		// or for send operations to block.
		time.Sleep(50 * time.Millisecond) // Give a moment for item1 to be processed internally

		cancel() // Cancel the context

		// Now try to read. The channel should close due to cancellation.
		// We might get item1 if it was already sent to 'out'.
		readCount := 0
	loop:
		for i := 0; i < 2; i++ { // Expect at most item1, then closure
			select {
			case _, ok := <-merged:
				if !ok {
					break loop // Channel closed, success
				}
				readCount++
			case <-time.After(200 * time.Millisecond):
				t.Errorf("merged channel did not close after cancellation with potentially blocked send. Read %d items.", readCount)
				break loop
			}
		}
		// Ensure ch1 can be closed without panic if the goroutine above is still trying to send.
		// This is a bit of a cleanup for the test itself.
		go func() {
			// Drain ch1 if anything is left, then close. This is to ensure the sender goroutine can exit.
			// This part is more about test hygiene than testing the Merge function itself.
			select {
			case <-ch1:
			default:
			}
			close(ch1)
		}()

	})
}

// Helper function to run tests for a given merge function implementation.
// It checks if all items from all input channels are received on the merged channel,
// and that the merged channel is eventually closed.
func runMergeTest[T comparable](t *testing.T, name string, inputs [][]T) {
	t.Run(name, func(t *testing.T) {
		t.Parallel() // Allow parallel execution of subtests

		ctx := context.Background() // Default context for non-cancellation tests

		inputChans := make([]<-chan T, len(inputs))
		expectedValues := make(map[T]int) // Using a map to count occurrences of each value
		totalExpectedCount := 0

		for i, items := range inputs {
			ch := make(chan T, len(items)) // Buffered channel for test setup
			for _, item := range items {
				ch <- item
				expectedValues[item]++
				totalExpectedCount++
			}
			close(ch)
			inputChans[i] = ch
		}

		// Call the merge function
		merged := feed.Merge(ctx, inputChans...)

		receivedValues := make(map[T]int)
		receivedCount := 0
		timeout := time.After(5 * time.Second) // Timeout to prevent test hanging

	loop:
		for {
			select {
			case val, ok := <-merged:
				if !ok { // Channel closed
					break loop
				}
				receivedValues[val]++
				receivedCount++
			case <-timeout:
				t.Fatalf("Test timed out waiting for merged channel to close in %s", name)
			}
		}

		if receivedCount != totalExpectedCount {
			t.Errorf("expected to receive %d values, but got %d in %s", totalExpectedCount, receivedCount, name)
		}

		if !reflect.DeepEqual(receivedValues, expectedValues) {
			t.Errorf("received values mismatch in %s.\nExpected (map): %v\nGot (map): %v",
				name, expectedValues, receivedValues)
		}
	})
}

// Helper function to create a slice of input items for testing.
func makeInput[T any](items ...T) []T {
	return items
}

func BenchmarkMerge(b *testing.B) {
	itemGenerator := func(i int) int { return i } // Simple int generator
	ctx := context.Background()                   // Benchmarks typically run without cancellation.

	for _, s := range []struct {
		numChannels     int
		itemsPerChannel int
	}{
		{1, 1},
		{1, 1000},
		{10, 10},
		{10, 100},
		{100, 10},
		{100, 100},
		{1000, 1},
	} {
		scenario := s // Capture range variable
		b.Run(fmt.Sprintf("Chan_%d_Item_%d", scenario.numChannels, scenario.itemsPerChannel), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer() // Stop timer for setup
				chansToMerge := generateChannels(scenario.numChannels, scenario.itemsPerChannel, itemGenerator)
				b.StartTimer() // Start timer for the merge operation itself

				merged := feed.Merge(ctx, chansToMerge...)
				consumeChannel(merged)
			}
		})
	}
}

// Helper to generate channels for benchmarking.
// Channels are buffered and pre-filled, then closed.
func generateChannels[T any](numChannels, itemsPerChannel int, itemGenerator func(i int) T) []<-chan T {
	channels := make([]<-chan T, numChannels)
	for i := 0; i < numChannels; i++ {
		ch := make(chan T, itemsPerChannel) // Buffered
		for j := 0; j < itemsPerChannel; j++ {
			ch <- itemGenerator(j)
		}
		close(ch)
		channels[i] = ch
	}
	return channels
}

// Helper to consume all items from a channel.
func consumeChannel[T any](ch <-chan T) {
	for range ch {
		// Just consume
	}
}
