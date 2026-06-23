package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"find-ten-game/internal/game"
)

const (
	gameIDByteCount          = 16
	defaultMaxStoredSessions = 150
)

var errSessionStoreFull = errors.New("session store is full")

type sessionStore struct {
	mu          sync.Mutex
	sessions    map[string]storedGame
	maxSessions int
}

type storedGame struct {
	session *game.GameSession
	broker  *snapshotBroker
}

func newSessionStore() *sessionStore {
	return &sessionStore{
		sessions:    make(map[string]storedGame),
		maxSessions: defaultMaxStoredSessions,
	}
}

func newStoredGame(session *game.GameSession) storedGame {
	return storedGame{
		session: session,
		broker:  newSnapshotBroker(session.Snapshots()),
	}
}

func (s *sessionStore) add(session *game.GameSession) (string, error) {
	stored := newStoredGame(session)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneCompletedLocked()
	if len(s.sessions) >= s.maxSessions {
		return "", errSessionStoreFull
	}

	for {
		id, err := newGameID()
		if err != nil {
			return "", err
		}
		if _, exists := s.sessions[id]; exists {
			continue
		}

		s.sessions[id] = stored
		return id, nil
	}
}

func (s *sessionStore) get(id string) (storedGame, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.sessions[id]
	return stored, ok
}

func (s *sessionStore) remove(id string) (storedGame, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.sessions[id]
	if !ok {
		return storedGame{}, false
	}

	delete(s.sessions, id)
	return stored, true
}

func (s *sessionStore) pruneCompletedLocked() {
	for id, stored := range s.sessions {
		select {
		case <-stored.session.Done():
			delete(s.sessions, id)
		default:
		}
	}
}

func newGameID() (string, error) {
	var bytes [gameIDByteCount]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes[:]), nil
}
