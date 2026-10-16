package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func linkedPostColumns(identity *clip.LinkedPostIdentity) (sql.NullString, sql.NullString) {
	if identity == nil || identity.Slug == "" {
		return sql.NullString{}, sql.NullString{}
	}
	return nullable(identity.Slug), nullable(stamp(identity.CreatedAt))
}

func decodeLinkedPostIdentity(slug, created sql.NullString) (*clip.LinkedPostIdentity, error) {
	if !slug.Valid && !created.Valid {
		return nil, nil
	}
	if !slug.Valid || !created.Valid || slug.String == "" {
		return nil, errors.New("stored linked post identity is incomplete")
	}
	at, err := time.Parse(time.RFC3339Nano, created.String)
	if err != nil {
		return nil, fmt.Errorf("stored linked post creation time: %w", err)
	}
	return &clip.LinkedPostIdentity{Slug: slug.String, CreatedAt: at}, nil
}
