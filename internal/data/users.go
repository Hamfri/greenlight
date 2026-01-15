package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"greenlight/internal/validator"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrDuplicateEmail = errors.New("duplicate email")
)

type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  password  `json:"-"`
	Activated bool      `json:"activated"`
	CreatedAt time.Time `json:"created_at"`
	Version   int       `json:"-"`
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

func ValidatePlainTextPassword(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes")

	// bcrypt processes a maximumm of 72 bytes and truncates anything that follows after
	// to process more than 72 characters consider pre-hashing passwords with SHA-256
	v.Check(len(password) <= 72, "password", "must be at most 72 bytes")
}

func ValidateUser(v *validator.Validator, user *User) map[string]string {
	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")

	ValidateEmail(v, user.Email)

	if user.Password.plaintext != nil {
		ValidatePlainTextPassword(v, *user.Password.plaintext)
	}

	// this should never happen
	// if it does there is a problem with our logic
	// for hashing user passwords
	if user.Password.hash == nil {
		panic("missing password hash for user")
	}

	if !v.Valid() {
		return v.Errors
	}

	return nil
}

// wraps connection pool
type UserModel struct {
	DB *sql.DB
}

func (m UserModel) Insert(user *User) error {
	query := `
		INSERT INTO users (name, email, password, activated)
		VALUES($1, $2, $3, $4)
		RETURNING id, created_at, version
	`
	args := []any{user.Name, user.Email, user.Password.hash, user.Activated}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

func (m UserModel) GetByEmail(email string) (*User, error) {
	query := `
		SELECT name, email, password, activated, created_at, version 
		FROM users 
		WHERE email = $1
	`

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	args := []any{}

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.Email,
		&user.Password,
		&user.Activated,
		&user.CreatedAt,
		&user.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

func (m UserModel) Update(user *User) error {
	query := `
		UPDATE users
		SET name = $1, email = $2, password = $3, activated = $4, version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING version
	`

	args := []any{
		user.Name,
		user.Email,
		user.Password.hash,
		user.Activated,
		user.ID,
		user.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (m UserModel) GetUserByToken(tokenScope, tokenPlainText string) (*User, error) {
	// calculate SHA-256 hash of the plaintext token provided by the client
	// returns a byte array with length 32, not a slice
	tokenHash := sha256.Sum256([]byte(tokenPlainText))

	query := `
		SELECT users.id, users.name, users.email, users.password, users.activated, users.created_at, users.version
		FROM users
		INNER JOIN tokens
		ON users.id = tokens.user_id
		WHERE tokens.hash = $1 -- vulnerable to timing attacks
		AND tokens.scope = $2
		AND tokens.expiry > $3
	`

	// [:] converts array to slice
	args := []any{tokenHash[:], tokenScope, time.Now()}

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.CreatedAt,
		&user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

type password struct {
	plaintext *string
	hash      []byte
}

func (p *password) Set(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	p.plaintext = &password
	p.hash = hash

	return nil
}

func (p *password) Matches(password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(password))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, err
		default:
			return false, err
		}

	}

	return true, nil
}
