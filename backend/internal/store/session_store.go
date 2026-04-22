package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/session"

type SessionStore interface {
	Create(s session.Session) error
	Get(token string) (session.Session, bool, error)
	Delete(token string) error
}
