package mail

import (
	"context"
	"log/slog"

	"github.com/postpilot/backend/internal/auth"
)

// Log is the local-development adapter. It makes every attempted message visible without
// requiring a provider account and keeps the exact same auth.Mailer contract as production.
type Log struct{}

func NewLog() *Log { return &Log{} }

func (*Log) Send(ctx context.Context, mail auth.Mail) error {
	slog.InfoContext(ctx, "transactional mail", "to", mail.To, "subject", mail.Subject, "body", mail.Text)
	return nil
}

var _ auth.Mailer = (*Log)(nil)
