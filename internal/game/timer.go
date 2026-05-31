package game

import (
	"context"
	"time"
)

func startExpiryTimer(ctx context.Context, expiresAt time.Time, expired chan<- struct{}) {
	timer := time.NewTimer(time.Until(expiresAt))
	defer timer.Stop()
	defer close(expired)

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		select {
		case <-ctx.Done():
		case expired <- struct{}{}:
		}
	}
}
