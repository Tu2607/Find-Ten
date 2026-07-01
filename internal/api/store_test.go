package api

import (
	"context"
	"errors"
	"testing"

	"find-ten-game/internal/game"
)

func TestSessionStoreRemoveDeletesSession(t *testing.T) {
	store := newSessionStore()
	session, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("NewGameSession returned error: %v", err)
	}
	defer session.Stop()

	id, err := store.add(session)
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}

	removed, ok := store.remove(id)
	if !ok {
		t.Fatal("remove returned ok=false, want true")
	}
	if removed.session != session {
		t.Fatal("removed session does not match stored session")
	}
	if _, ok := store.get(id); ok {
		t.Fatal("get returned deleted session, want missing")
	}
}

func TestSessionStoreAddPrunesCompletedSessions(t *testing.T) {
	store := newSessionStore()

	completed, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("NewGameSession returned error: %v", err)
	}
	completedID, err := store.add(completed)
	if err != nil {
		t.Fatalf("add completed candidate returned error: %v", err)
	}
	completed.Stop()
	<-completed.Done()

	active, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("NewGameSession returned error: %v", err)
	}
	defer active.Stop()

	if _, err := store.add(active); err != nil {
		t.Fatalf("add active session returned error: %v", err)
	}

	if _, ok := store.get(completedID); ok {
		t.Fatal("completed session remained in store, want pruned")
	}
}

func TestSessionStoreAddKeepsActiveSessions(t *testing.T) {
	store := newSessionStore()

	active, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("NewGameSession returned error: %v", err)
	}
	defer active.Stop()

	activeID, err := store.add(active)
	if err != nil {
		t.Fatalf("add active session returned error: %v", err)
	}

	another, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("NewGameSession returned error: %v", err)
	}
	defer another.Stop()

	if _, err := store.add(another); err != nil {
		t.Fatalf("add second session returned error: %v", err)
	}

	if _, ok := store.get(activeID); !ok {
		t.Fatal("active session was removed during pruning")
	}
}

func TestSessionStoreAddSucceedsWhenPruneFreesCapacity(t *testing.T) {
	store := newSessionStore()
	store.maxSessions = 1

	completed, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("NewGameSession returned error: %v", err)
	}
	if _, err := store.add(completed); err != nil {
		t.Fatalf("add completed candidate returned error: %v", err)
	}
	completed.Stop()
	<-completed.Done()

	replacement, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("NewGameSession returned error: %v", err)
	}
	defer replacement.Stop()

	if _, err := store.add(replacement); err != nil {
		t.Fatalf("add replacement returned error: %v", err)
	}
}

func TestSessionStoreAddFailsWhenActiveSessionsReachCapacity(t *testing.T) {
	store := newSessionStore()
	store.maxSessions = 1

	active, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("NewGameSession returned error: %v", err)
	}
	defer active.Stop()

	if _, err := store.add(active); err != nil {
		t.Fatalf("add active session returned error: %v", err)
	}

	extra, _, err := game.NewGameSession(context.Background(), 9, game.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("NewGameSession returned error: %v", err)
	}
	defer extra.Stop()

	if _, err := store.add(extra); !errors.Is(err, errSessionStoreFull) {
		t.Fatalf("add extra error = %v, want %v", err, errSessionStoreFull)
	}
}
