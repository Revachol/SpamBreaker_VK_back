package domain

import "time"

type BMessage struct {
	Text   string    `json:"text"`
	SendAt time.Time `json:"send_at"`
}
