// Package events fans out state-change events to the open /events streams.
package events

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
)

var (
	ErrTooManySubscribers = errors.New("too many event subscribers")
	ErrClosed             = errors.New("event broker closed")
)

// Event is a named SSE event with its JSON payload, marshalled once.
type Event struct {
	Name string
	Data []byte
}

// Broker never blocks publishers: a subscriber whose buffer is full is
// dropped (its channel closed) and is expected to reconnect and resync from
// the snapshot.
type Broker struct {
	mu     sync.Mutex
	subs   map[chan Event]struct{}
	max    int
	closed bool
	log    *slog.Logger
}

const bufferSize = 64

func NewBroker(maxSubscribers int, log *slog.Logger) *Broker {
	if log == nil {
		log = slog.Default()
	}
	return &Broker{subs: map[chan Event]struct{}{}, max: maxSubscribers, log: log.With("component", "events")}
}

// Subscribe returns a channel of events, closed when the subscriber is
// dropped or the broker closes, and a function to unsubscribe.
func (b *Broker) Subscribe() (<-chan Event, func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, nil, ErrClosed
	}
	if len(b.subs) >= b.max {
		return nil, nil, ErrTooManySubscribers
	}
	ch := make(chan Event, bufferSize)
	b.subs[ch] = struct{}{}
	return ch, func() { b.remove(ch) }, nil
}

func (b *Broker) remove(ch chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
}

func (b *Broker) Publish(name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		b.log.Error("marshal event", "event", name, "err", err)
		return
	}
	ev := Event{Name: name, Data: data}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
			delete(b.subs, ch)
			close(ch)
			b.log.Warn("dropped slow subscriber")
		}
	}
}

// Close ends every subscription; used on shutdown so SSE handlers return.
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for ch := range b.subs {
		delete(b.subs, ch)
		close(ch)
	}
}
