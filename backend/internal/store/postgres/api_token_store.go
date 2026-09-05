package postgres

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

type APITokenStore struct{ Store *Store }

func (s *APITokenStore) Put(token apitoken.Token) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Store.db.ExecContext(ctx, `
		INSERT INTO api_tokens (id, user_id, name, secret_hash, created_at, expires_at, scopes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		token.ID, token.UserID, token.Name, token.SecretHash, token.CreatedAt.UTC(), token.ExpiresAt,
		strings.Join(token.Scopes, ","))
	return err
}

func (s *APITokenStore) Get(id string) (apitoken.Token, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := s.Store.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, secret_hash, created_at, expires_at, revoked_at, last_used_at, scopes
		FROM api_tokens WHERE id = $1`, id)
	token, err := scanToken(row)
	if err == sql.ErrNoRows {
		return apitoken.Token{}, false, nil
	}
	return token, err == nil, err
}

func (s *APITokenStore) ListByUser(userID string) ([]apitoken.Token, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rows, err := s.Store.db.QueryContext(ctx, `
		SELECT id, user_id, name, secret_hash, created_at, expires_at, revoked_at, last_used_at, scopes
		FROM api_tokens WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []apitoken.Token{}
	for rows.Next() {
		token, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, token)
	}
	return out, rows.Err()
}

func (s *APITokenStore) Revoke(id, userID string, at time.Time) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The owner is part of the condition, so a caller cannot revoke somebody
	// else's token by guessing an identifier.
	result, err := s.Store.db.ExecContext(ctx, `
		UPDATE api_tokens SET revoked_at = $3
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, id, userID, at.UTC())
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (s *APITokenStore) Touch(id string, at time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Store.db.ExecContext(ctx,
		`UPDATE api_tokens SET last_used_at = $2 WHERE id = $1`, id, at.UTC())
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanToken(row scanner) (apitoken.Token, error) {
	var token apitoken.Token
	var expiresAt, revokedAt, lastUsedAt sql.NullTime
	var scopes string
	if err := row.Scan(&token.ID, &token.UserID, &token.Name, &token.SecretHash,
		&token.CreatedAt, &expiresAt, &revokedAt, &lastUsedAt, &scopes); err != nil {
		return apitoken.Token{}, err
	}
	if trimmed := strings.TrimSpace(scopes); trimmed != "" {
		token.Scopes = strings.Split(trimmed, ",")
	}
	for stamp, target := range map[*sql.NullTime]**time.Time{
		&expiresAt: &token.ExpiresAt, &revokedAt: &token.RevokedAt, &lastUsedAt: &token.LastUsedAt,
	} {
		if stamp.Valid {
			value := stamp.Time.UTC()
			*target = &value
		}
	}
	return token, nil
}
