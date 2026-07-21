package player

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	sqlitedriver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	MinDisplayNameLength       = 3
	MaxDisplayNameLength       = 10
	MinPasswordLength          = 12
	MaxPasswordBytes           = 72
	handleSuffixDigits         = 6
	maxHandleGenerationRetries = 8
)

var (
	ErrInvalidDisplayName        = errors.New("invalid display name")
	ErrInvalidPassword           = errors.New("invalid password")
	ErrPlayerNotFound            = errors.New("player not found")
	ErrSessionNotFound           = errors.New("player session not found")
	ErrInvalidCredentials        = errors.New("invalid account credentials")
	ErrHandleGenerationExhausted = errors.New("account handle generation exhausted")
)

type Store struct {
	db           *sql.DB
	now          func() time.Time
	handleSuffix func() (string, error)
}

type Account struct {
	ID            int64
	DisplayName   string
	AccountHandle string
	CreatedAt     time.Time
}

type CreateAccountInput struct {
	DisplayName string
	Password    string
}

func NewStore(ctx context.Context, db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("new player store: database is required")
	}

	store := &Store{
		db:           db,
		now:          time.Now,
		handleSuffix: randomHandleSuffix,
	}
	if err := store.init(ctx); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	// Player stores share the process-owned SQLite DB. This hook is a no-op
	// until later account work adds resources owned by the store itself.
	return nil
}

func ValidateDisplayName(displayName string) error {
	switch {
	case displayName == "":
		return fmt.Errorf("%w: display name is required", ErrInvalidDisplayName)
	case utf8.RuneCountInString(displayName) < MinDisplayNameLength:
		return fmt.Errorf("%w: display name must be at least %d characters", ErrInvalidDisplayName, MinDisplayNameLength)
	case utf8.RuneCountInString(displayName) > MaxDisplayNameLength:
		return fmt.Errorf("%w: display name must be at most %d characters", ErrInvalidDisplayName, MaxDisplayNameLength)
	case !isSafeDisplayName(displayName):
		return fmt.Errorf("%w: display name must include an ASCII letter or digit and use only ASCII letters, digits, spaces, hyphen, underscore, or apostrophe", ErrInvalidDisplayName)
	default:
		return nil
	}
}

func (s *Store) CreateAccount(ctx context.Context, input CreateAccountInput) (Account, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	if err := ValidateDisplayName(displayName); err != nil {
		return Account{}, err
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return Account{}, err
	}

	for attempt := 0; attempt <= maxHandleGenerationRetries; attempt++ {
		accountHandle := displayName
		if attempt > 0 {
			suffix, err := s.handleSuffix()
			if err != nil {
				return Account{}, fmt.Errorf("generate account handle suffix: %w", err)
			}
			accountHandle = displayName + "#" + suffix
		}

		account, err := s.insertAccount(ctx, displayName, accountHandle, passwordHash)
		if err == nil {
			return account, nil
		}
		if !isUniqueConstraint(err) {
			return Account{}, err
		}
	}

	return Account{}, ErrHandleGenerationExhausted
}

func (s *Store) FindAccountByHandle(ctx context.Context, accountHandle string) (Account, error) {
	account, _, err := s.findAccountByHandle(ctx, accountHandle)
	return account, err
}

func (s *Store) Authenticate(ctx context.Context, accountHandle, password string) (Account, error) {
	if err := validatePasswordForAuthentication(password); err != nil {
		return Account{}, ErrInvalidCredentials
	}

	account, passwordHash, err := s.findAccountByHandle(ctx, accountHandle)
	if err != nil {
		if errors.Is(err, ErrPlayerNotFound) {
			consumePasswordVerificationTime(password)
			return Account{}, ErrInvalidCredentials
		}
		return Account{}, err
	}
	if !VerifyPassword(passwordHash, password) {
		return Account{}, ErrInvalidCredentials
	}

	return account, nil
}

func (s *Store) insertAccount(ctx context.Context, displayName, accountHandle, passwordHash string) (Account, error) {
	createdAt := s.now().UTC()
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)
	`, displayName, accountHandle, passwordHash, formatTime(createdAt))
	if err != nil {
		return Account{}, fmt.Errorf("insert player account: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Account{}, fmt.Errorf("read player account ID: %w", err)
	}

	return Account{
		ID:            id,
		DisplayName:   displayName,
		AccountHandle: accountHandle,
		CreatedAt:     createdAt,
	}, nil
}

func (s *Store) findAccountByHandle(ctx context.Context, accountHandle string) (Account, string, error) {
	var account Account
	var passwordHash string
	var createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, display_name, account_handle, password_hash, created_at
		FROM players
		WHERE account_handle = ?
	`, accountHandle).Scan(
		&account.ID,
		&account.DisplayName,
		&account.AccountHandle,
		&passwordHash,
		&createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, "", ErrPlayerNotFound
	}
	if err != nil {
		return Account{}, "", fmt.Errorf("find player account by handle: %w", err)
	}

	account.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return Account{}, "", fmt.Errorf("parse player account created_at: %w", err)
	}

	return account, passwordHash, nil
}

func isSafeDisplayName(value string) bool {
	hasLetterOrDigit := false
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
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

func isUniqueConstraint(err error) bool {
	var sqliteErr *sqlitedriver.Error
	// account_handle is currently the only application-defined UNIQUE column.
	// Update this classifier if future schema changes add another one.
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

func randomHandleSuffix() (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(handleSuffixDigits), nil)
	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%0*d", handleSuffixDigits, value.Int64()), nil
}

func (s *Store) init(ctx context.Context) error {
	for _, statement := range schemaStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize player schema: %w", err)
		}
	}

	return nil
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS players (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 3 AND 10),
		account_handle TEXT NOT NULL UNIQUE CHECK (length(account_handle) BETWEEN 1 AND 32),
		password_hash TEXT NOT NULL CHECK (length(password_hash) BETWEEN 1 AND 255),
		created_at TEXT NOT NULL CHECK (
			created_at GLOB '????-??-??T*Z'
			AND datetime(created_at) IS NOT NULL
		)
	)`,
	`CREATE TABLE IF NOT EXISTS player_sessions (
		token_hash TEXT PRIMARY KEY CHECK (length(token_hash) = 64),
		player_id INTEGER NOT NULL REFERENCES players(id) ON DELETE CASCADE,
		created_at TEXT NOT NULL CHECK (
			created_at GLOB '????-??-??T*Z'
			AND datetime(created_at) IS NOT NULL
		),
		expires_at TEXT NOT NULL CHECK (
			expires_at GLOB '????-??-??T*Z'
			AND datetime(expires_at) IS NOT NULL
		)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_player_sessions_player_id
	ON player_sessions (player_id)`,
	`CREATE INDEX IF NOT EXISTS idx_player_sessions_expires_at
	ON player_sessions (expires_at)`,
}
