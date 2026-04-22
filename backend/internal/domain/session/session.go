package session

import "time"

type Session struct {
	Token     string
	Identity  string
	ExpiresAt time.Time
}
