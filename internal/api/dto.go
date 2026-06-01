package api

import (
	"time"

	"find-ten-game/internal/game"
)

type createGameRequest struct {
	Size *int `json:"size"`
}

type createGameResponse struct {
	GameID          string           `json:"gameId"`
	InitialSnapshot snapshotResponse `json:"initialSnapshot"`
	ExpiresAt       time.Time        `json:"expiresAt"`
}

type snapshotResponse struct {
	Sequence       int64     `json:"sequence"`
	Board          [][]int   `json:"board"`
	Score          int       `json:"score"`
	GameOver       bool      `json:"gameOver"`
	GameOverReason int       `json:"gameOverReason"`
	ValidMoveCount int       `json:"validMoveCount"`
	SnapshotTime   time.Time `json:"snapshotTime"`
}

func newSnapshotResponse(snapshot game.GameSnapshot) snapshotResponse {
	return snapshotResponse{
		Sequence:       snapshot.Sequence,
		Board:          [][]int(snapshot.Board),
		Score:          snapshot.Score,
		GameOver:       snapshot.GameOver,
		GameOverReason: int(snapshot.GameOverReason),
		ValidMoveCount: snapshot.ValidMoveCount,
		SnapshotTime:   snapshot.SnapshotTime,
	}
}
