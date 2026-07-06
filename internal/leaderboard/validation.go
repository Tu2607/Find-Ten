package leaderboard

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"find-ten-game/internal/game"
)

const (
	// Future score-submission API handlers should server-stamp submissions.
	// Five minutes leaves room for small host clock adjustments while rejecting
	// bogus future timestamps from tests, tools, or future import paths.
	maxFutureSubmittedAtSkew = 5 * time.Minute
	MaxGameIDLength          = 64
	MaxPlayerNameLength      = 10
)

func normalizeSubmission(submission ScoreSubmission) ScoreSubmission {
	submission.PlayerName = strings.TrimSpace(submission.PlayerName)
	return submission
}

func validateSubmission(submission ScoreSubmission, now time.Time) error {
	switch {
	case submission.GameID == "":
		return fmt.Errorf("%w: game ID is required", ErrInvalidScoreSubmission)
	case len(submission.GameID) > MaxGameIDLength:
		return fmt.Errorf("%w: game ID must be at most %d characters", ErrInvalidScoreSubmission, MaxGameIDLength)
	case submission.PlayerName == "":
		return fmt.Errorf("%w: player name is required", ErrInvalidScoreSubmission)
	case utf8.RuneCountInString(submission.PlayerName) > MaxPlayerNameLength:
		return fmt.Errorf("%w: player name must be at most %d characters", ErrInvalidScoreSubmission, MaxPlayerNameLength)
	case !isSafePlayerName(submission.PlayerName):
		return fmt.Errorf("%w: player name must include an ASCII letter or digit and use only ASCII letters, digits, spaces, hyphen, underscore, or apostrophe", ErrInvalidScoreSubmission)
	case game.ValidateBoardSize(submission.GridSize) != nil:
		return fmt.Errorf("%w: unsupported grid size", ErrInvalidScoreSubmission)
	case game.ValidateDuration(submission.DurationSeconds) != nil:
		return fmt.Errorf("%w: unsupported duration seconds", ErrInvalidScoreSubmission)
	case submission.Score < 0:
		return fmt.Errorf("%w: score must be non-negative", ErrInvalidScoreSubmission)
	case submission.RemainingMillis < 0:
		return fmt.Errorf("%w: remaining millis must be non-negative", ErrInvalidScoreSubmission)
	case submission.RemainingMillis > submission.DurationSeconds*1000:
		return fmt.Errorf("%w: remaining millis exceeds game duration", ErrInvalidScoreSubmission)
	case submission.SubmittedAt.IsZero():
		return fmt.Errorf("%w: submitted at is required", ErrInvalidScoreSubmission)
	case submission.SubmittedAt.After(now.Add(maxFutureSubmittedAtSkew)):
		return fmt.Errorf("%w: submitted at is too far in the future", ErrInvalidScoreSubmission)
	default:
		return nil
	}
}

func isSafePlayerName(value string) bool {
	// Keep stored names display-safe by default. The WebUI can accept these
	// same characters without later HTML-specific sanitization: ASCII letters,
	// digits, spaces, hyphen, underscore, and apostrophe.
	hasLetterOrDigit := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			hasLetterOrDigit = true
			continue
		}
		if r >= 'A' && r <= 'Z' {
			hasLetterOrDigit = true
			continue
		}
		if r >= '0' && r <= '9' {
			hasLetterOrDigit = true
			continue
		}
		if r == ' ' || r == '-' || r == '_' || r == '\'' {
			continue
		}
		return false
	}

	return hasLetterOrDigit
}
