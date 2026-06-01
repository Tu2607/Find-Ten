package api

import (
	"testing"
	"time"

	"find-ten-game/internal/game"
)

func TestSnapshotBrokerPublishesToMultipleSubscribers(t *testing.T) {
	source := make(chan game.GameSnapshot)
	broker := newSnapshotBroker(source)
	defer close(source)

	first, unsubscribeFirst, ok := broker.subscribe()
	if !ok {
		t.Fatal("first subscribe failed, want active broker")
	}
	defer unsubscribeFirst()

	second, unsubscribeSecond, ok := broker.subscribe()
	if !ok {
		t.Fatal("second subscribe failed, want active broker")
	}
	defer unsubscribeSecond()

	source <- game.GameSnapshot{Sequence: 2}

	assertSnapshotSequence(t, first, 2)
	assertSnapshotSequence(t, second, 2)
}

func TestSnapshotBrokerLateSubscriberReceivesLatestRuntimeSnapshot(t *testing.T) {
	source := make(chan game.GameSnapshot)
	broker := newSnapshotBroker(source)
	defer close(source)

	source <- game.GameSnapshot{Sequence: 2}

	late, unsubscribe, ok := broker.subscribe()
	if !ok {
		t.Fatal("late subscribe failed, want active broker")
	}
	defer unsubscribe()

	assertSnapshotSequence(t, late, 2)
}

func TestSnapshotBrokerSlowSubscriberKeepsLatestSnapshot(t *testing.T) {
	broker := &snapshotBroker{
		subscribers: make(map[chan snapshotResponse]struct{}),
		done:        make(chan struct{}),
	}

	subscriber, unsubscribe, ok := broker.subscribe()
	if !ok {
		t.Fatal("subscribe failed, want active broker")
	}
	defer unsubscribe()

	broker.publish(snapshotResponse{Sequence: 2})
	broker.publish(snapshotResponse{Sequence: 3})

	assertSnapshotSequence(t, subscriber, 3)
}

func TestSnapshotBrokerClosesSubscribersWhenSourceCloses(t *testing.T) {
	source := make(chan game.GameSnapshot)
	broker := newSnapshotBroker(source)

	subscriber, unsubscribe, ok := broker.subscribe()
	if !ok {
		t.Fatal("subscribe failed, want active broker")
	}
	defer unsubscribe()

	close(source)

	assertChannelClosed(t, subscriber)
	assertDoneClosed(t, broker.done)

	if _, _, ok := broker.subscribe(); ok {
		t.Fatal("subscribe after broker close succeeded, want failure")
	}
}

func TestSnapshotBrokerUnsubscribeRemovesSubscriberWithoutClosing(t *testing.T) {
	source := make(chan game.GameSnapshot)
	broker := newSnapshotBroker(source)
	defer close(source)

	subscriber, unsubscribe, ok := broker.subscribe()
	if !ok {
		t.Fatal("subscribe failed, want active broker")
	}

	unsubscribe()

	select {
	case _, ok := <-subscriber:
		if !ok {
			t.Fatal("subscriber channel closed by unsubscribe, want open channel")
		}
	default:
	}

	source <- game.GameSnapshot{Sequence: 2}

	select {
	case snapshot := <-subscriber:
		t.Fatalf("unsubscribed channel received snapshot %+v", snapshot)
	case <-time.After(20 * time.Millisecond):
	}
}

func assertSnapshotSequence(t *testing.T, snapshots <-chan snapshotResponse, want int64) {
	t.Helper()

	select {
	case snapshot, ok := <-snapshots:
		if !ok {
			t.Fatal("snapshot channel closed, want snapshot")
		}
		if snapshot.Sequence != want {
			t.Fatalf("snapshot.Sequence = %d, want %d", snapshot.Sequence, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for snapshot sequence %d", want)
	}
}

func assertChannelClosed(t *testing.T, snapshots <-chan snapshotResponse) {
	t.Helper()

	select {
	case _, ok := <-snapshots:
		if ok {
			t.Fatal("snapshot channel is open, want closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscriber channel to close")
	}
}

func assertDoneClosed(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broker done channel to close")
	}
}
