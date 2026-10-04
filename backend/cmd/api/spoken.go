package main

import (
	"context"
	"database/sql"
	jobstore "github.com/postpilot/backend/internal/job/store"
	modelcatalogapp "github.com/postpilot/backend/internal/modelcatalog/app"
	"github.com/postpilot/backend/internal/voice/spoken"
	spokenapp "github.com/postpilot/backend/internal/voice/spoken/app"
	spokenstore "github.com/postpilot/backend/internal/voice/spoken/store"
)

type spokenTransactions struct{ writer *sql.DB }

func (s spokenTransactions) Write(ctx context.Context, fn func(spokenapp.TxPorts) error) error {
	tx, err := s.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(spokenapp.TxPorts{Operations: spokenstore.NewTx(tx), Jobs: jobstore.NewTx(tx, jobKinds())}); err != nil {
		return err
	}
	return tx.Commit()
}

type spokenPublishLibraries struct{ base spokenapp.GenerationDeps }

func (s spokenPublishLibraries) LibraryForOperation(id string) *spoken.Service {
	return spoken.NewService(spokenapp.PublicationStorage{OperationStorage: s.base.Operations, Transactions: s.base.Transactions, OperationID: id}, s.base.Profiles, s.base.Objects)
}
func newSpokenGeneration(c *contexts) *spokenapp.GenerationService {
	p := c.platform
	d := spokenapp.GenerationDeps{Library: c.spoken, Operations: spokenstore.New(p.db.Writer, p.db.Reader), Profiles: spokenProfiles{p.speechCatalog}, Prices: modelcatalogapp.SpeechBudgets{Profiles: p.speechCatalog}, Ledger: c.ledger, Models: c.metered, Jobs: spokenapp.NewJobs(c.jobs), Transactions: spokenTransactions{p.db.Writer}, Objects: p.bucket}
	d.Publisher = spokenPublishLibraries{d}
	return spokenapp.NewGenerationService(d)
}
