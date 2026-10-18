package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/storage"
	"github.com/alexagc/kalyx/internal/storage/migrations"
	_ "modernc.org/sqlite"
)

func init() {
	storage.RegisterDriver("sqlite", func(ctx context.Context, dsn string) (storage.Storage, error) {
		return New(ctx, dsn)
	})
}

// Storage implements storage.Storage using pure-Go CGO-free SQLite.
type Storage struct {
	db *sql.DB
}

// New opens a connection to SQLite, configures WAL mode and connection pooling,
// and automatically runs all pending embedded migrations.
func New(ctx context.Context, dsn string) (*Storage, error) {
	if dsn == "" {
		dsn = "kalyx.db"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Connection pool settings for SQLite with WAL
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Configure SQLite performance and integrity PRAGMAs
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA synchronous = NORMAL;",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to apply pragma %s: %w", pragma, err)
		}
	}

	// Auto-apply embedded SQL migrations
	if err := migrations.RunSQLiteMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to apply sqlite migrations: %w", err)
	}

	return &Storage{db: db}, nil
}

func (s *Storage) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) ListEvents(ctx context.Context) ([]core.EventDefinition, error) {
	query := `SELECT id, domain, name, type, description, owner_team, created_at, updated_at FROM events ORDER BY name ASC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	events := make([]core.EventDefinition, 0)
	for rows.Next() {
		var ev core.EventDefinition
		var desc sql.NullString
		if err := rows.Scan(&ev.ID, &ev.Domain, &ev.Name, &ev.Type, &desc, &ev.OwnerTeam, &ev.CreatedAt, &ev.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		if desc.Valid {
			ev.Description = desc.String
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}

func (s *Storage) GetEvent(ctx context.Context, id string) (*core.EventDefinition, error) {
	query := `SELECT id, domain, name, type, description, owner_team, created_at, updated_at FROM events WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var ev core.EventDefinition
	var desc sql.NullString
	if err := row.Scan(&ev.ID, &ev.Domain, &ev.Name, &ev.Type, &desc, &ev.OwnerTeam, &ev.CreatedAt, &ev.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrEventNotFound
		}
		return nil, fmt.Errorf("failed to get event %s: %w", id, err)
	}
	if desc.Valid {
		ev.Description = desc.String
	}
	return &ev, nil
}

func (s *Storage) CreateEvent(ctx context.Context, event core.EventDefinition) error {
	now := time.Now().UTC()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	if event.UpdatedAt.IsZero() {
		event.UpdatedAt = now
	}

	query := `INSERT INTO events (id, domain, name, type, description, owner_team, created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, event.ID, event.Domain, event.Name, event.Type, event.Description, event.OwnerTeam, event.CreatedAt, event.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return storage.ErrConflict
		}
		return fmt.Errorf("failed to create event: %w", err)
	}
	return nil
}

func (s *Storage) ListVersions(ctx context.Context, eventID string) ([]core.EventVersion, error) {
	query := `SELECT id, event_id, semver, status, json_schema, target_table, partition_by, pii_fields, created_by, approved_by, approved_at, created_at 
	          FROM event_versions WHERE event_id = ? ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query versions: %w", err)
	}
	defer rows.Close()

	versions := make([]core.EventVersion, 0)
	for rows.Next() {
		var v core.EventVersion
		var targetTable, partitionBy, piiFields, approvedBy sql.NullString
		var approvedAt sql.NullTime
		if err := rows.Scan(&v.ID, &v.EventID, &v.SemVer, &v.Status, &v.JSONSchema, &targetTable, &partitionBy, &piiFields, &v.CreatedBy, &approvedBy, &approvedAt, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan version: %w", err)
		}
		if targetTable.Valid {
			v.DataLakeMetadata.TargetTable = targetTable.String
		}
		if partitionBy.Valid && partitionBy.String != "" {
			_ = json.Unmarshal([]byte(partitionBy.String), &v.DataLakeMetadata.PartitionBy)
		}
		if piiFields.Valid && piiFields.String != "" {
			_ = json.Unmarshal([]byte(piiFields.String), &v.DataLakeMetadata.PIIFields)
		}
		if approvedBy.Valid {
			v.ApprovedBy = approvedBy.String
		}
		if approvedAt.Valid {
			v.ApprovedAt = &approvedAt.Time
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

func (s *Storage) GetEventVersion(ctx context.Context, eventID, semver string) (*core.EventVersion, error) {
	query := `SELECT id, event_id, semver, status, json_schema, target_table, partition_by, pii_fields, created_by, approved_by, approved_at, created_at 
	          FROM event_versions WHERE event_id = ? AND semver = ?`
	row := s.db.QueryRowContext(ctx, query, eventID, semver)

	var v core.EventVersion
	var targetTable, partitionBy, piiFields, approvedBy sql.NullString
	var approvedAt sql.NullTime
	if err := row.Scan(&v.ID, &v.EventID, &v.SemVer, &v.Status, &v.JSONSchema, &targetTable, &partitionBy, &piiFields, &v.CreatedBy, &approvedBy, &approvedAt, &v.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrVersionNotFound
		}
		return nil, fmt.Errorf("failed to get version: %w", err)
	}
	if targetTable.Valid {
		v.DataLakeMetadata.TargetTable = targetTable.String
	}
	if partitionBy.Valid && partitionBy.String != "" {
		_ = json.Unmarshal([]byte(partitionBy.String), &v.DataLakeMetadata.PartitionBy)
	}
	if piiFields.Valid && piiFields.String != "" {
		_ = json.Unmarshal([]byte(piiFields.String), &v.DataLakeMetadata.PIIFields)
	}
	if approvedBy.Valid {
		v.ApprovedBy = approvedBy.String
	}
	if approvedAt.Valid {
		v.ApprovedAt = &approvedAt.Time
	}
	return &v, nil
}

func (s *Storage) CreateEventVersion(ctx context.Context, version core.EventVersion) error {
	now := time.Now().UTC()
	if version.CreatedAt.IsZero() {
		version.CreatedAt = now
	}

	partsJSON, _ := json.Marshal(version.DataLakeMetadata.PartitionBy)
	piiJSON, _ := json.Marshal(version.DataLakeMetadata.PIIFields)

	query := `INSERT INTO event_versions (id, event_id, semver, status, json_schema, target_table, partition_by, pii_fields, created_by, approved_by, approved_at, created_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, version.ID, version.EventID, version.SemVer, version.Status, version.JSONSchema,
		version.DataLakeMetadata.TargetTable, string(partsJSON), string(piiJSON), version.CreatedBy, version.ApprovedBy, version.ApprovedAt, version.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return storage.ErrConflict
		}
		return fmt.Errorf("failed to create version: %w", err)
	}
	return nil
}

func (s *Storage) UpdateEventVersionStatus(ctx context.Context, eventID, semver string, status core.EventStatus) error {
	query := `UPDATE event_versions SET status = ? WHERE event_id = ? AND semver = ?`
	res, err := s.db.ExecContext(ctx, query, status, eventID, semver)
	if err != nil {
		return fmt.Errorf("failed to update version status: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return storage.ErrVersionNotFound
	}
	return nil
}

func (s *Storage) ListProposals(ctx context.Context, status *core.EventStatus) ([]core.ChangeProposal, error) {
	var query string
	var args []interface{}
	if status != nil {
		query = `SELECT id, event_id, target_semver, proposed_schema, proposed_by, status, review_notes, is_breaking, breaking_reason, created_at 
		         FROM change_proposals WHERE status = ? ORDER BY created_at DESC`
		args = append(args, *status)
	} else {
		query = `SELECT id, event_id, target_semver, proposed_schema, proposed_by, status, review_notes, is_breaking, breaking_reason, created_at 
		         FROM change_proposals ORDER BY created_at DESC`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query proposals: %w", err)
	}
	defer rows.Close()

	proposals := make([]core.ChangeProposal, 0)
	for rows.Next() {
		var p core.ChangeProposal
		var reviewNotes, breakingReason sql.NullString
		var isBreaking int
		if err := rows.Scan(&p.ID, &p.EventID, &p.TargetSemVer, &p.ProposedSchema, &p.ProposedBy, &p.Status, &reviewNotes, &isBreaking, &breakingReason, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan proposal: %w", err)
		}
		p.IsBreaking = isBreaking == 1
		if reviewNotes.Valid {
			p.ReviewNotes = reviewNotes.String
		}
		if breakingReason.Valid {
			p.BreakingReason = breakingReason.String
		}
		proposals = append(proposals, p)
	}
	return proposals, rows.Err()
}

func (s *Storage) GetProposal(ctx context.Context, id string) (*core.ChangeProposal, error) {
	query := `SELECT id, event_id, target_semver, proposed_schema, proposed_by, status, review_notes, is_breaking, breaking_reason, created_at 
	          FROM change_proposals WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var p core.ChangeProposal
	var reviewNotes, breakingReason sql.NullString
	var isBreaking int
	if err := row.Scan(&p.ID, &p.EventID, &p.TargetSemVer, &p.ProposedSchema, &p.ProposedBy, &p.Status, &reviewNotes, &isBreaking, &breakingReason, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrProposalNotFound
		}
		return nil, fmt.Errorf("failed to get proposal: %w", err)
	}
	p.IsBreaking = isBreaking == 1
	if reviewNotes.Valid {
		p.ReviewNotes = reviewNotes.String
	}
	if breakingReason.Valid {
		p.BreakingReason = breakingReason.String
	}
	return &p, nil
}

func (s *Storage) CreateProposal(ctx context.Context, proposal core.ChangeProposal) error {
	now := time.Now().UTC()
	if proposal.CreatedAt.IsZero() {
		proposal.CreatedAt = now
	}

	isBreakingInt := 0
	if proposal.IsBreaking {
		isBreakingInt = 1
	}

	query := `INSERT INTO change_proposals (id, event_id, target_semver, proposed_schema, proposed_by, status, review_notes, is_breaking, breaking_reason, created_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, proposal.ID, proposal.EventID, proposal.TargetSemVer, proposal.ProposedSchema, proposal.ProposedBy,
		proposal.Status, proposal.ReviewNotes, isBreakingInt, proposal.BreakingReason, proposal.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return storage.ErrConflict
		}
		return fmt.Errorf("failed to create proposal: %w", err)
	}
	return nil
}

func (s *Storage) UpdateProposalStatus(ctx context.Context, id string, status core.EventStatus, reviewer, notes string) error {
	query := `UPDATE change_proposals SET status = ?, review_notes = ? WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, status, notes, id)
	if err != nil {
		return fmt.Errorf("failed to update proposal status: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return storage.ErrProposalNotFound
	}
	return nil
}

func (s *Storage) SavePAT(ctx context.Context, token core.PATToken) error {
	now := time.Now().UTC()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = now
	}

	query := `INSERT INTO pat_tokens (id, name, token_hash, owner_email, team, expires_at, created_at, last_used_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, token.ID, token.Name, token.TokenHash, token.OwnerEmail, token.Team, token.ExpiresAt, token.CreatedAt, token.LastUsedAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return storage.ErrConflict
		}
		return fmt.Errorf("failed to save PAT: %w", err)
	}
	return nil
}

func (s *Storage) GetPATByHash(ctx context.Context, tokenHash string) (*core.PATToken, error) {
	query := `SELECT id, name, token_hash, owner_email, team, expires_at, created_at, last_used_at FROM pat_tokens WHERE token_hash = ?`
	row := s.db.QueryRowContext(ctx, query, tokenHash)

	var token core.PATToken
	var expiresAt, lastUsedAt sql.NullTime
	if err := row.Scan(&token.ID, &token.Name, &token.TokenHash, &token.OwnerEmail, &token.Team, &expiresAt, &token.CreatedAt, &lastUsedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrTokenNotFound
		}
		return nil, fmt.Errorf("failed to get PAT by hash: %w", err)
	}
	if expiresAt.Valid {
		token.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		token.LastUsedAt = &lastUsedAt.Time
	}
	return &token, nil
}

func (s *Storage) ListPATs(ctx context.Context, ownerEmail string) ([]core.PATToken, error) {
	var query string
	var args []interface{}
	if ownerEmail != "" {
		query = `SELECT id, name, token_hash, owner_email, team, expires_at, created_at, last_used_at FROM pat_tokens WHERE owner_email = ? ORDER BY created_at DESC`
		args = append(args, ownerEmail)
	} else {
		query = `SELECT id, name, token_hash, owner_email, team, expires_at, created_at, last_used_at FROM pat_tokens ORDER BY created_at DESC`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list PATs: %w", err)
	}
	defer rows.Close()

	tokens := make([]core.PATToken, 0)
	for rows.Next() {
		var token core.PATToken
		var expiresAt, lastUsedAt sql.NullTime
		if err := rows.Scan(&token.ID, &token.Name, &token.TokenHash, &token.OwnerEmail, &token.Team, &expiresAt, &token.CreatedAt, &lastUsedAt); err != nil {
			return nil, fmt.Errorf("failed to scan token: %w", err)
		}
		if expiresAt.Valid {
			token.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			token.LastUsedAt = &lastUsedAt.Time
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (s *Storage) RevokePAT(ctx context.Context, id string) error {
	query := `DELETE FROM pat_tokens WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to revoke PAT: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return storage.ErrTokenNotFound
	}
	return nil
}

func (s *Storage) TouchPAT(ctx context.Context, id string) error {
	now := time.Now().UTC()
	query := `UPDATE pat_tokens SET last_used_at = ? WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("failed to touch PAT: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return storage.ErrTokenNotFound
	}
	return nil
}
