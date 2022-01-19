# Topic based Fan-Out Subscriptions

[![Go](https://github.com/janos/feed/workflows/Go/badge.svg)](https://github.com/janos/feed/actions)
[![PkgGoDev](https://pkg.go.dev/badge/resenje.org/feed)](https://pkg.go.dev/resenje.org/feed)
[![NewReleases](https://newreleases.io/badge.svg)](https://newreleases.io/github/janos/feed)

This Go module provides fan-out synchronization methods using topic based dynamic subscriptions. Two types are available: Feed and Trigger.

## Feed

Feed is sending the same message to all subscribers. All messages are sent with preserved order and the delivery is guaranteed.

Example usage where topics are strings and messages some of a defined struct type.

```go
// define feed message type
type Message struct {
	ID   int
	Text string
}

f := feed.NewFeed[string, Message]()
defer f.Close()

// subscribe to the feed topic "info"
subscription1, cancel1 := f.Subscribe("info")
defer cancel1()

go func() {
	for m := range subscription1 {
		fmt.Println("subscription 1 got message", m)
	}
}()

// subscribe to the feed topic "info", again
subscription2, cancel2 := f.Subscribe("info")
defer cancel2()

go func() {
	for m := range subscription2 {
		fmt.Println("subscription 2 got message", m)
	}
}()


// send some messages to both subscriptions
f.Send("info", Message{
	ID:   25,
	Text: "Hello, there.",
})

f.Send("info", Message{
	ID:   101,
	Text: "Something happened",
})

f.Send("info", Message{
	ID:   106,
	Text: "Something else happened",
})
```

## Trigger

Trigger is notifying subscribers if an event happened. If multiple events have happened, only one trigger event will be received by subscribers. This approach is ensuring that no unnecessary repetition of events happens and that appropriate action is done once after the event is received.


```go
t := feed.NewTrigger[int]()

// subscribe to topic 100
subscription1, cancel1 := t.Subscribe(100)
defer cancel1()

go func() {
	for range subscription1 {
		// do something
	}
}()

// subscribe to topic 100
subscription2, cancel2 := t.Subscribe(100)
defer cancel2()

go func() {
	for range subscription2 {
		// do something
	}
}()

// send a few events to the topic 1
t.Trigger(100)
t.Trigger(100)
t.Trigger(100)
```

## Installation

Run `go get -u resenje.org/feed` from command line.

## License

This application is distributed under the BSD-style license found in the [LICENSE](LICENSE) file.
