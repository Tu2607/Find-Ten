package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"find-ten-game/internal/player"
)

func newPlayerResponse(account player.Account) playerResponse {
	return playerResponse{
		DisplayName:   account.DisplayName,
		AccountHandle: account.AccountHandle,
	}
}

func (s *Server) authenticatedPlayer(r *http.Request) (player.Account, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return player.Account{}, player.ErrSessionNotFound
	}

	return s.players.FindAccountBySessionToken(r.Context(), cookie.Value)
}

func (s *Server) authenticatedPlayerForScore(r *http.Request) (player.Account, bool, error) {
	account, err := s.authenticatedPlayer(r)
	if errors.Is(err, player.ErrSessionNotFound) {
		return player.Account{}, false, nil
	}
	if err != nil {
		return player.Account{}, false, err
	}

	return account, true, nil
}

func setSessionCookie(w http.ResponseWriter, token string) {
	now := time.Now()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  now.Add(player.SessionLifetime),
		MaxAge:   int(player.SessionLifetime.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

func writePlayerCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, player.ErrInvalidDisplayName), errors.Is(err, player.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, player.ErrHandleGenerationExhausted):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	default:
		writePlayerStorageError(w, err, "failed to create player")
	}
}

func writeLoginError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, player.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid account handle or password")
	default:
		writePlayerStorageError(w, err, "failed to log in")
	}
}

func writeCurrentPlayerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, player.ErrSessionNotFound):
		writeError(w, http.StatusUnauthorized, "authentication required")
	default:
		writePlayerStorageError(w, err, "failed to look up current player")
	}
}

func writePlayerStorageError(w http.ResponseWriter, err error, message string) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		writeError(w, http.StatusRequestTimeout, err.Error())
		return
	}

	writeError(w, http.StatusInternalServerError, message)
}
