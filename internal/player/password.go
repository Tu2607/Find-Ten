package player

import (
	"fmt"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// dummyPasswordHash is a valid bcrypt.DefaultCost hash used to keep
// unknown-handle login attempts on the same bcrypt work path as
// wrong-password attempts. Update it if HashPassword changes bcrypt cost.
const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func ValidatePassword(password string) error {
	switch {
	case len(password) > MaxPasswordBytes:
		return fmt.Errorf("%w: password must be at most %d bytes", ErrInvalidPassword, MaxPasswordBytes)
	case utf8.RuneCountInString(password) < MinPasswordLength:
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidPassword, MinPasswordLength)
	default:
		return nil
	}
}

func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

func VerifyPassword(passwordHash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
}

func consumePasswordVerificationTime(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
}
