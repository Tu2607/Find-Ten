package player

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var accountTestNow = time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

func TestCreateAccountStoresBcryptHashAndFindsAccount(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	account, err := store.CreateAccount(ctx, CreateAccountInput{
		DisplayName: "  Ada  ",
		Password:    "correct-password",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	if account.DisplayName != "Ada" {
		t.Fatalf("DisplayName = %q, want Ada", account.DisplayName)
	}
	if account.AccountHandle != "Ada" {
		t.Fatalf("AccountHandle = %q, want Ada", account.AccountHandle)
	}
	if account.CreatedAt != accountTestNow {
		t.Fatalf("CreatedAt = %s, want %s", account.CreatedAt, accountTestNow)
	}

	var passwordHash string
	if err := store.db.QueryRowContext(ctx, `SELECT password_hash FROM players WHERE id = ?`, account.ID).Scan(&passwordHash); err != nil {
		t.Fatalf("password hash lookup failed: %v", err)
	}
	if passwordHash == "correct-password" {
		t.Fatal("password hash stores plaintext password")
	}
	if !VerifyPassword(passwordHash, "correct-password") {
		t.Fatal("stored password hash does not verify")
	}

	found, err := store.FindAccountByHandle(ctx, "Ada")
	if err != nil {
		t.Fatalf("FindAccountByHandle failed: %v", err)
	}
	if found != account {
		t.Fatalf("found account = %#v, want %#v", found, account)
	}
}

func TestCreateAccountSamePasswordUsesDistinctBcryptHashes(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	first, err := store.CreateAccount(ctx, CreateAccountInput{DisplayName: "Ada", Password: "correct-password"})
	if err != nil {
		t.Fatalf("first CreateAccount failed: %v", err)
	}
	second, err := store.CreateAccount(ctx, CreateAccountInput{DisplayName: "Grace", Password: "correct-password"})
	if err != nil {
		t.Fatalf("second CreateAccount failed: %v", err)
	}

	var firstHash, secondHash string
	if err := store.db.QueryRowContext(ctx, `SELECT password_hash FROM players WHERE id = ?`, first.ID).Scan(&firstHash); err != nil {
		t.Fatalf("first hash lookup failed: %v", err)
	}
	if err := store.db.QueryRowContext(ctx, `SELECT password_hash FROM players WHERE id = ?`, second.ID).Scan(&secondHash); err != nil {
		t.Fatalf("second hash lookup failed: %v", err)
	}
	if firstHash == secondHash {
		t.Fatal("same password stored with identical bcrypt hashes")
	}
}

func TestAuthenticate(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	created, err := store.CreateAccount(ctx, CreateAccountInput{DisplayName: "Ada", Password: "correct-password"})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	tests := []struct {
		name     string
		handle   string
		password string
		wantErr  error
	}{
		{name: "correct password", handle: "Ada", password: "correct-password"},
		{name: "wrong password", handle: "Ada", password: "wrong-password", wantErr: ErrInvalidCredentials},
		{name: "unknown handle", handle: "Unknown", password: "correct-password", wantErr: ErrInvalidCredentials},
		{name: "case-variant handle", handle: "ada", password: "correct-password", wantErr: ErrInvalidCredentials},
		{name: "invalid password", handle: "Ada", password: "short", wantErr: ErrInvalidCredentials},
		{name: "oversized password", handle: "Ada", password: strings.Repeat("a", MaxPasswordBytes+1), wantErr: ErrInvalidCredentials},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account, err := store.Authenticate(ctx, test.handle, test.password)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Authenticate error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && account != created {
				t.Fatalf("Authenticate account = %#v, want %#v", account, created)
			}
		})
	}
}

func TestFindAccountByHandleMissing(t *testing.T) {
	_, err := openTestStore(t).FindAccountByHandle(context.Background(), "Unknown")
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("FindAccountByHandle error = %v, want %v", err, ErrPlayerNotFound)
	}
}

func TestCreateAccountGeneratesUniqueHandlesAfterCollision(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	values := []string{"100001", "100002"}
	store.handleSuffix = func() (string, error) {
		value := values[0]
		values = values[1:]
		return value, nil
	}

	first, err := store.CreateAccount(ctx, CreateAccountInput{DisplayName: "Ada", Password: "correct-password"})
	if err != nil {
		t.Fatalf("first CreateAccount failed: %v", err)
	}
	second, err := store.CreateAccount(ctx, CreateAccountInput{DisplayName: "Ada", Password: "correct-password"})
	if err != nil {
		t.Fatalf("second CreateAccount failed: %v", err)
	}
	third, err := store.CreateAccount(ctx, CreateAccountInput{DisplayName: "Ada", Password: "correct-password"})
	if err != nil {
		t.Fatalf("third CreateAccount failed: %v", err)
	}

	if first.AccountHandle != "Ada" {
		t.Fatalf("first handle = %q, want Ada", first.AccountHandle)
	}
	if second.AccountHandle != "Ada#100001" {
		t.Fatalf("second handle = %q, want Ada#100001", second.AccountHandle)
	}
	if third.AccountHandle != "Ada#100002" {
		t.Fatalf("third handle = %q, want Ada#100002", third.AccountHandle)
	}
}

func TestCreateAccountReturnsControlledErrorWhenHandleRetriesExhausted(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	store.handleSuffix = func() (string, error) { return "100001", nil }

	if _, err := store.CreateAccount(ctx, CreateAccountInput{DisplayName: "Ada", Password: "correct-password"}); err != nil {
		t.Fatalf("first CreateAccount failed: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `
		INSERT INTO players (display_name, account_handle, password_hash, created_at)
		VALUES (?, ?, ?, ?)
	`, "Other", "Ada#100001", "hash", formatTime(accountTestNow)); err != nil {
		t.Fatalf("seed colliding handle failed: %v", err)
	}

	_, err := store.CreateAccount(ctx, CreateAccountInput{DisplayName: "Ada", Password: "correct-password"})
	if !errors.Is(err, ErrHandleGenerationExhausted) {
		t.Fatalf("CreateAccount error = %v, want %v", err, ErrHandleGenerationExhausted)
	}
}

func TestValidateDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  error
	}{
		{name: "minimum length", value: "Ada"},
		{name: "maximum length", value: "TenLetters"},
		{name: "safe punctuation", value: "A-D_1' B"},
		{name: "empty", value: "", want: ErrInvalidDisplayName},
		{name: "too short", value: "Al", want: ErrInvalidDisplayName},
		{name: "too long", value: "ElevenChars", want: ErrInvalidDisplayName},
		{name: "unsafe newline", value: "Ada\nBob", want: ErrInvalidDisplayName},
		{name: "unsafe markup", value: "<script>", want: ErrInvalidDisplayName},
		{name: "punctuation only", value: "---", want: ErrInvalidDisplayName},
		{name: "non ASCII", value: "Ada界", want: ErrInvalidDisplayName},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateDisplayName(test.value)
			if !errors.Is(err, test.want) {
				t.Fatalf("ValidateDisplayName(%q) error = %v, want %v", test.value, err, test.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  error
	}{
		{name: "minimum length", value: "12345678"},
		{name: "multibyte within byte limit", value: strings.Repeat("界", 8)},
		{name: "empty", value: "", want: ErrInvalidPassword},
		{name: "too short", value: "1234567", want: ErrInvalidPassword},
		{name: "over bcrypt byte limit", value: strings.Repeat("a", MaxPasswordBytes+1), want: ErrInvalidPassword},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidatePassword(test.value)
			if !errors.Is(err, test.want) {
				t.Fatalf("ValidatePassword error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestHashPasswordRejectsInvalidPassword(t *testing.T) {
	_, err := HashPassword("short")
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("HashPassword error = %v, want %v", err, ErrInvalidPassword)
	}
}

func TestVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if !VerifyPassword(hash, "correct-password") {
		t.Fatal("VerifyPassword rejected the correct password")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("VerifyPassword accepted the wrong password")
	}
	if VerifyPassword(hash, "") {
		t.Fatal("VerifyPassword accepted the empty password")
	}
}

func TestCreateAccountTrimsBeforeValidatingDisplayName(t *testing.T) {
	_, err := openTestStore(t).CreateAccount(context.Background(), CreateAccountInput{
		DisplayName: "  Ab  ",
		Password:    "correct-password",
	})
	if !errors.Is(err, ErrInvalidDisplayName) {
		t.Fatalf("CreateAccount error = %v, want %v", err, ErrInvalidDisplayName)
	}
}

func TestCreateAccountRejectsInvalidPassword(t *testing.T) {
	_, err := openTestStore(t).CreateAccount(context.Background(), CreateAccountInput{
		DisplayName: "Ada",
		Password:    "short",
	})
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("CreateAccount error = %v, want %v", err, ErrInvalidPassword)
	}
}

func TestAuthenticatePropagatesStorageErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := openTestStore(t).Authenticate(ctx, "Ada", "correct-password")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Authenticate error = %v, want %v", err, context.Canceled)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := NewStore(context.Background(), openTestDB(t))
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	store.now = func() time.Time { return accountTestNow }
	return store
}
