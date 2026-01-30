package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"greenlight/internal/validator"
	"time"
)

// consts for token scope.
const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
	ScopePasswordReset  = "password-reset"
)

func ValidatePlainTextToken(v *validator.Validator, token string) {
	v.Check(len(token) == 26, "token", "must be 26 bytes long")
}

type Token struct {
	PlainText string    `json:"token"`
	Hash      []byte    `json:"-"`
	userID    int       `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

func generateToken(userID int, ttl time.Duration, scope string) *Token {
	token := &Token{
		PlainText: rand.Text(),
		userID:    userID,
		Expiry:    time.Now().Add(ttl),
		Scope:     scope,
	}

	hash := sha256.Sum256([]byte(token.PlainText))
	token.Hash = hash[:]
	return token
}

type TokenModel struct {
	DB *sql.DB
}

func (m TokenModel) New(userID int, ttl time.Duration, scope string) (*Token, error) {
	token := generateToken(userID, ttl, scope)

	err := m.Insert(token)
	return token, err
}

func (m TokenModel) Insert(token *Token) error {
	query := `
		INSERT INTO tokens (hash, user_id, expiry, scope)
		VALUES ($1, $2, $3, $4)
	`

	args := []any{token.Hash, token.userID, token.Expiry, token.Scope}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	return nil
}

func (m TokenModel) Delete(scope string, userID int) error {
	query := `
		DELETE FROM tokens 
		WHERE scope = $1 
		AND user_id = $2
	`

	args := []any{scope, userID}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, args...)

	return err
}
