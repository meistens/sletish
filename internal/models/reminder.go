package models

import "time"

type Reminder struct {
	ID             int       `json:"id"`
	UserID         string    `json:"user_id"`
	MediaID        int       `json:"media_id"`
	Message        string    `json:"message"`
	RemindAt       time.Time `json:"remind_at"`
	Sent           bool      `json:"sent"`
	CreatedAt      time.Time `json:"created_at"`
	MediaTitle     string    `json:"media_title,omitempty"`
	MediaPosterURL string    `json:"media_poster_url,omitempty"`
}
