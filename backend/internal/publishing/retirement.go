package publishing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const RetirementMigrationVersion = 72

type RetirementSource interface {
	RetirementSnapshot(ctx context.Context) (RetirementSnapshot, error)
}

type RetirementSnapshot struct {
	CutoffAt     string
	Agents       []RetirementAgent
	Jobs         []RetirementJob
	Pairings     int64
	Reservations int64
	Assets       int64
}

type RetirementAgent struct {
	ID                   string `json:"id"`
	AccountID            string `json:"account_id"`
	Label                string `json:"label"`
	PlatformAccountID    string `json:"platform_account_id,omitempty"`
	PlatformAccountLabel string `json:"platform_account_label,omitempty"`
	RevokedAt            string `json:"revoked_at"`
}

type RetirementJob struct {
	ID              string `json:"id"`
	AccountID       string `json:"account_id"`
	PostSlug        string `json:"post_slug"`
	PostCreatedAt   string `json:"post_created_at"`
	Status          string `json:"status"`
	Stage           string `json:"stage"`
	CommittedAt     string `json:"committed_at,omitempty"`
	PublishedAt     string `json:"published_at,omitempty"`
	FailureReason   string `json:"failure_reason,omitempty"`
	PlatformPostURL string `json:"platform_post_url,omitempty"`
}

type RetirementCounts struct {
	Pairings     int64 `json:"pairings"`
	Agents       int64 `json:"agents"`
	Reservations int64 `json:"job_id_reservations"`
	Jobs         int64 `json:"jobs"`
	Assets       int64 `json:"assets"`
}

type RetirementReport struct {
	SchemaVersion    int               `json:"schema_version"`
	MigrationVersion int               `json:"migration_version"`
	Environment      string            `json:"environment"`
	DatabaseIdentity string            `json:"database_identity"`
	CutoffAt         string            `json:"cutoff_at"`
	Counts           RetirementCounts  `json:"counts"`
	Agents           []RetirementAgent `json:"agents"`
	Jobs             []RetirementJob   `json:"jobs"`
	Digest           string            `json:"digest"`
}

func BuildRetirementReport(ctx context.Context, source RetirementSource, environment, databaseIdentity string) (RetirementReport, error) {
	environment = strings.TrimSpace(environment)
	databaseIdentity = strings.TrimSpace(databaseIdentity)
	if environment == "" || databaseIdentity == "" {
		return RetirementReport{}, errors.New("retirement environment and database identity are required")
	}
	snapshot, err := source.RetirementSnapshot(ctx)
	if err != nil {
		return RetirementReport{}, fmt.Errorf("read retirement snapshot: %w", err)
	}
	if snapshot.CutoffAt == "" {
		return RetirementReport{}, errors.New("publishing retirement migration is not applied")
	}
	report := RetirementReport{
		SchemaVersion: 1, MigrationVersion: RetirementMigrationVersion,
		Environment: environment, DatabaseIdentity: databaseIdentity, CutoffAt: snapshot.CutoffAt,
		Counts: RetirementCounts{Pairings: snapshot.Pairings, Agents: int64(len(snapshot.Agents)), Reservations: snapshot.Reservations, Jobs: int64(len(snapshot.Jobs)), Assets: snapshot.Assets},
		Agents: snapshot.Agents, Jobs: snapshot.Jobs,
	}
	digestInput, err := json.Marshal(report)
	if err != nil {
		return RetirementReport{}, err
	}
	digest := sha256.Sum256(digestInput)
	report.Digest = hex.EncodeToString(digest[:])
	return report, nil
}

func DatabaseIdentity(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	digest := sha256.Sum256([]byte("sqlite:" + filepath.Clean(absolute)))
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

// WriteRetirementReport publishes complete bytes through a same-directory hard link.
// The destination therefore appears atomically and an existing original is never replaced.
func WriteRetirementReport(path string, report RetirementReport) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("retirement report path is required")
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode retirement report: %w", err)
	}
	payload = append(payload, '\n')
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".retirepublishing-*.tmp")
	if err != nil {
		return fmt.Errorf("create retirement report temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("protect retirement report: %w", err)
	}
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return fmt.Errorf("write retirement report: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync retirement report: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close retirement report: %w", err)
	}
	if err := os.Link(tmpPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("retirement report already exists: %s", path)
		}
		return fmt.Errorf("publish retirement report: %w", err)
	}
	directory, err := os.Open(dir)
	if err == nil {
		err = directory.Sync()
		directory.Close()
	}
	if err != nil {
		return fmt.Errorf("sync retirement report directory: %w", err)
	}
	return nil
}
