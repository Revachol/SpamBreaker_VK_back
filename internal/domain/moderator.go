package domain

import "time"

// Moderator — пользователь панели управления.
type Moderator struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	PasswordHash string    `json:"-"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ModeratorAccount struct {
	ID                string     `db:"id" json:"id"`
	ModeratorID       string     `db:"moderator_id" json:"moderator_id"`
	Platform          string     `db:"platform" json:"platform"`
	AccountID         *string    `db:"account_id" json:"account_id"`
	VerificationToken *string    `db:"verification_token" json:"verification_token,omitempty"`
	TokenExpiresAt    *time.Time `db:"token_expires_at" json:"token_expires_at,omitempty"`
	VerifiedAt        *time.Time `db:"verified_at" json:"verified_at,omitempty"`
}
