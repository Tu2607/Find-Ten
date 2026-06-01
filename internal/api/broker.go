package api

import (
	"sync"

	"find-ten-game/internal/game"
)

const snapshotSubscriberBuffer = 1

type snapshotBroker struct {
	source <-chan game.GameSnapshot

	mu          sync.Mutex
	subscribers map[chan snapshotResponse]struct{}
	latest      snapshotResponse
	hasLatest   bool
	closed      bool
	done        chan struct{}
}

func newSnapshotBroker(source <-chan game.GameSnapshot) *snapshotBroker {
	broker := &snapshotBroker{
		source:      source,
		subscribers: make(map[chan snapshotResponse]struct{}),
		done:        make(chan struct{}),
	}
	go broker.run()
	return broker
}

func (b *snapshotBroker) run() {
	// This blocks until RunGame sends a runtime snapshot or closes the source channel.
	for snapshot := range b.source {
		b.publish(newSnapshotResponse(snapshot))
	}
	b.close()
}

func (b *snapshotBroker) subscribe() (<-chan snapshotResponse, func(), bool) {
	ch := make(chan snapshotResponse, snapshotSubscriberBuffer)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, nil, false
	}

	b.subscribers[ch] = struct{}{}
	if b.hasLatest {
		ch <- b.latest
	}

	unsubscribe := func() {
		b.unsubscribe(ch)
	}

	return ch, unsubscribe, true
}

func (b *snapshotBroker) unsubscribe(ch <-chan snapshotResponse) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for subscriber := range b.subscribers {
		if (<-chan snapshotResponse)(subscriber) == ch {
			delete(b.subscribers, subscriber)
			return
		}
	}
}

func (b *snapshotBroker) publish(snapshot snapshotResponse) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.latest = snapshot
	b.hasLatest = true

	for subscriber := range b.subscribers {
		select {
		case subscriber <- snapshot:
		default:
			select {
			case <-subscriber:
			default:
			}
			subscriber <- snapshot
		}
	}
}

func (b *snapshotBroker) close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.closed = true
	close(b.done)
	for subscriber := range b.subscribers {
		close(subscriber)
		delete(b.subscribers, subscriber)
	}
}
