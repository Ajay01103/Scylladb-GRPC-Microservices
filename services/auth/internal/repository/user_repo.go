package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gocql/gocql"
)

// User represents a user record
type User struct {
	ID        string    `db:"id"`
	Email     string    `db:"email"`
	Name      string    `db:"name"`
	Password  string    `db:"password"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ErrUserNotFound is returned when a user lookup yields no result.
var ErrUserNotFound = errors.New("user not found")

// ErrEmailTaken is returned when registering with an already-used email.
var ErrEmailTaken = errors.New("email already in use")

// ErrNameTaken is returned when registering with an already-used name.
var ErrNameTaken = errors.New("name already in use")

// UserRepo provides data access for users using ScyllaDB
type UserRepo struct {
	session *gocql.Session
}

// NewUserRepo creates a UserRepo backed by a ScyllaDB session
func NewUserRepo(session *gocql.Session) *UserRepo {
	return &UserRepo{session: session}
}

// CreateUser inserts a new user record. Detects unique-constraint violations.
func (r *UserRepo) CreateUser(ctx context.Context, email, name, hashedPassword string) (User, error) {
	userID := uuid.New().String()
	now := time.Now()

	// First, check if email is already taken
	var existingEmail string
	err := r.session.Query(
		"SELECT email FROM users WHERE email = ? LIMIT 1",
		email,
	).WithContext(ctx).Scan(&existingEmail)
	if err == nil {
		// Email found, so it's already taken
		return User{}, ErrEmailTaken
	}
	if err != gocql.ErrNotFound {
		return User{}, fmt.Errorf("check email: %w", err)
	}

	// Check if name is already taken
	var existingName string
	err = r.session.Query(
		"SELECT name FROM users WHERE name = ? LIMIT 1",
		name,
	).WithContext(ctx).Scan(&existingName)
	if err == nil {
		// Name found, so it's already taken
		return User{}, ErrNameTaken
	}
	if err != gocql.ErrNotFound {
		return User{}, fmt.Errorf("check name: %w", err)
	}

	// Insert the new user
	err = r.session.Query(
		`INSERT INTO users (id, email, name, password, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		userID, email, name, hashedPassword, now, now,
	).WithContext(ctx).Exec()
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	return User{
		ID:        userID,
		Email:     email,
		Name:      name,
		Password:  hashedPassword,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetByEmail fetches a user by email
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := r.session.Query(
		`SELECT id, email, name, password, created_at, updated_at
		FROM users WHERE email = ? LIMIT 1`,
		email,
	).WithContext(ctx).Scan(
		&user.ID, &user.Email, &user.Name, &user.Password,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == gocql.ErrNotFound {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

// GetByID fetches a user by UUID
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	var user User
	err := r.session.Query(
		`SELECT id, email, name, password, created_at, updated_at
		FROM users WHERE id = ? LIMIT 1`,
		id.String(),
	).WithContext(ctx).Scan(
		&user.ID, &user.Email, &user.Name, &user.Password,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == gocql.ErrNotFound {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}
