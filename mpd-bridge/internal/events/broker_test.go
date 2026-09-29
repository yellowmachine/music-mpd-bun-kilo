package events

import (
	"errors"
	"testing"
)

func TestPublishAndSlowSubscriberDropped(t *testing.T) {
	b := NewBroker(2, nil)
	fast, unsubFast, _ := b.Subscribe()
	slow, _, _ := b.Subscribe()
	defer unsubFast()

	for i := 0; i <= bufferSize; i++ {
		b.Publish("player", map[string]int{"i": i})
		<-fast
	}
	// slow never read: its buffer filled and it was dropped.
	n := 0
	for range slow {
		n++
	}
	if n != bufferSize {
		t.Errorf("slow got %d buffered events before close, want %d", n, bufferSize)
	}

	b.Publish("mixer", map[string]int{"volume": 5})
	if ev := <-fast; ev.Name != "mixer" || string(ev.Data) != `{"volume":5}` {
		t.Errorf("got %s %s", ev.Name, ev.Data)
	}
}

func TestLimitAndClose(t *testing.T) {
	b := NewBroker(1, nil)
	ch, _, err := b.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.Subscribe(); !errors.Is(err, ErrTooManySubscribers) {
		t.Errorf("got %v", err)
	}
	b.Close()
	if _, ok := <-ch; ok {
		t.Error("channel still open after Close")
	}
	if _, _, err := b.Subscribe(); !errors.Is(err, ErrClosed) {
		t.Errorf("got %v", err)
	}
}
