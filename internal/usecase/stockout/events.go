package stockout

import (
	"sync"
	"time"
)

// Event tells open count screens that the numbers changed, so they reload
// the summary at once instead of waiting for their next poll.
type Event struct {
	Type    string    `json:"type"` // batch, void, review, config
	Date    string    `json:"date"` // Bangkok day the change belongs to
	BatchID uint      `json:"batchId,omitempty"`
	At      time.Time `json:"at"`
}

// Broker fans events out to every open stream in this process. Several API
// processes behind a load balancer each have their own broker; screens fall
// back to polling, so a missed event only means a few seconds' delay.
type Broker struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewBroker builds an empty broker.
func NewBroker() *Broker { return &Broker{subs: map[chan Event]struct{}{}} }

// Subscribe returns a channel of events and a function that closes it.
func (b *Broker) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, ch)
			b.mu.Unlock()
			close(ch)
		})
	}
}

// Publish never blocks: a stream that is not keeping up skips the event.
func (b *Broker) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribers is the number of open streams.
func (b *Broker) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}
