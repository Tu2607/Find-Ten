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

type submitMoveRequest struct {
	Selection *selectionRequest `json:"selection"`
}

type submitRemoveNumberRequest struct {
	Position *positionRequest `json:"position"`
}

type selectionRequest struct {
	Start *positionRequest `json:"start"`
	End   *positionRequest `json:"end"`
}

type positionRequest struct {
	Row *int `json:"row"`
	Col *int `json:"col"`
}

type submitMoveResponse struct {
	Accepted bool `json:"accepted"`
}

type snapshotResponse struct {
	Sequence         int64     `json:"sequence"`
	Board            [][]int   `json:"board"`
	Score            int       `json:"score"`
	GameOver         bool      `json:"gameOver"`
	GameOverReason   int       `json:"gameOverReason"`
	ValidMoveCount   int       `json:"validMoveCount"`
	ReshuffleUsed    bool      `json:"reshuffleUsed"`
	RemoveNumberUsed bool      `json:"removeNumberUsed"`
	SnapshotTime     time.Time `json:"snapshotTime"`
}

func newSnapshotResponse(snapshot game.GameSnapshot) snapshotResponse {
	return snapshotResponse{
		Sequence:         snapshot.Sequence,
		Board:            [][]int(snapshot.Board),
		Score:            snapshot.Score,
		GameOver:         snapshot.GameOver,
		GameOverReason:   int(snapshot.GameOverReason),
		ValidMoveCount:   snapshot.ValidMoveCount,
		ReshuffleUsed:    snapshot.ReshuffleUsed,
		RemoveNumberUsed: snapshot.RemoveNumberUsed,
		SnapshotTime:     snapshot.SnapshotTime,
	}
}

// Converts the API move request to a game.Selection. Returns false if the request is invalid.
func (r submitMoveRequest) toSelection() (game.Selection, bool) {
	if r.Selection == nil || r.Selection.Start == nil || r.Selection.End == nil {
		return game.Selection{}, false
	}
	if r.Selection.Start.Row == nil || r.Selection.Start.Col == nil {
		return game.Selection{}, false
	}
	if r.Selection.End.Row == nil || r.Selection.End.Col == nil {
		return game.Selection{}, false
	}

	return game.Selection{
		Start: game.Position{
			Row: *r.Selection.Start.Row,
			Col: *r.Selection.Start.Col,
		},
		End: game.Position{
			Row: *r.Selection.End.Row,
			Col: *r.Selection.End.Col,
		},
	}, true
}

func (r submitRemoveNumberRequest) toPosition() (game.Position, bool) {
	if r.Position == nil || r.Position.Row == nil || r.Position.Col == nil {
		return game.Position{}, false
	}

	return game.Position{
		Row: *r.Position.Row,
		Col: *r.Position.Col,
	}, true
}
