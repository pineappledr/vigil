package events

import (
	"log"
	"sync"
	"time"
)

// Handler is a callback invoked when a matching event is published.
type Handler func(Event)

// subscription ties a handler to the event types it cares about.
type subscription struct {
	types   map[EventType]struct{} // nil means "all events"
	handler Handler
}

// Bus is a thread-safe, in-process publish/subscribe event bus.
type Bus struct {
	mu          sync.RWMutex
	subscribers []subscription
}

// NewBus creates a ready-to-use event bus.
func NewBus() *Bus {
	return &Bus{}
}

// Subscribe registers a handler for the given event types.
// If no types are provided the handler receives every event.
func (b *Bus) Subscribe(handler Handler, types ...EventType) {
	sub := subscription{handler: handler}
	if len(types) > 0 {
		sub.types = make(map[EventType]struct{}, len(types))
		for _, t := range types {
			sub.types[t] = struct{}{}
		}
	}

	b.mu.Lock()
	b.subscribers = append(b.subscribers, sub)
	b.mu.Unlock()
}

// Publish sends an event to all matching subscribers.
// The timestamp is set automatically if zero.
// Handlers are called synchronously in the caller's goroutine;
// the dispatcher (Task 1.2) is responsible for its own concurrency.
func (b *Bus) Publish(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	b.mu.RLock()
	subs := make([]subscription, len(b.subscribers))
	copy(subs, b.subscribers)
	b.mu.RUnlock()

	for _, sub := range subs {
		if sub.types != nil {
			if _, ok := sub.types[e.Type]; !ok {
				continue
			}
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("events: subscriber panic on %s: %v", e.Type, r)
				}
			}()
			sub.handler(e)
		}()
	}
}
