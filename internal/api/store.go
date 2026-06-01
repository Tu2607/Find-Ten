package api

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"find-ten-game/internal/game"
)

const gameIDByteCount = 16

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]storedGame
}

type storedGame struct {
	session *game.GameSession
	broker  *snapshotBroker
}

func newSessionStore() *sessionStore {
	return &sessionStore{
		sessions: make(map[string]storedGame),
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

func newGameID() (string, error) {
	var bytes [gameIDByteCount]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes[:]), nil
}
