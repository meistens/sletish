package models

import "time"

type Status string

const (
	StatusWatching  Status = "watching"
	StatusCompleted Status = "completed"
	StatusOnHold    Status = "on_hold"
	StatusDropped   Status = "dropped"
	StatusWatchlist Status = "watchlist"
)

type AppUser struct {
	ID        string    `json:"id" db:"id" validate:"required"`
	Username  *string   `json:"username" db:"username" validate:"max=50"`
	Platform  string    `json:"platform" db:"platform" validate:"required,oneof=telegram"` // **NOTE:MODIFY FOR FUTURE PLATFORMS**
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Media struct {
	ID          int       `json:"id" db:"id"`
	ExternalID  string    `json:"external_id" db:"external_id"`
	Title       string    `json:"title" db:"title"`
	Type        string    `json:"type" db:"type"`
	Description *string   `json:"description" db:"description"`
	ReleaseDate *string   `json:"release_date" db:"release_date"`
	PosterURL   *string   `json:"poster_url" db:"poster_url"`
	Rating      *float64  `json:"rating" db:"rating"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type UserMedia struct {
	ID        int       `json:"id" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	MediaID   int       `json:"media_id" db:"media_id"`
	Status    Status    `json:"status" db:"status"`
	Rating    *float64  `json:"rating" db:"rating"`
	Notes     *string   `json:"notes" db:"notes"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type UserMediaWithDetails struct {
	UserMedia UserMedia `json:"user_media"`
	Media     Media     `json:"media"`
}
