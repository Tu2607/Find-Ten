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
	sessions map[string]*game.GameSession
}

func newSessionStore() *sessionStore {
	return &sessionStore{
		sessions: make(map[string]*game.GameSession),
	}
}

func (s *sessionStore) add(session *game.GameSession) (string, error) {
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

		s.sessions[id] = session
		return id, nil
	}
}

func (s *sessionStore) get(id string) (*game.GameSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	return session, ok
}

func newGameID() (string, error) {
	var bytes [gameIDByteCount]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes[:]), nil
}
