package cloudrunner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// SessionStore is the minimal DB interface needed for cloud runtime continuity.
type SessionStore interface {
	GetCloudRuntimeSessionForIssue(ctx context.Context, arg db.GetCloudRuntimeSessionForIssueParams) (db.CloudRuntimeSession, error)
	GetCloudRuntimeSessionForChat(ctx context.Context, arg db.GetCloudRuntimeSessionForChatParams) (db.CloudRuntimeSession, error)
	UpsertCloudRuntimeSession(ctx context.Context, arg db.UpsertCloudRuntimeSessionParams) (db.UpsertCloudRuntimeSessionRow, error)
}

type SessionService struct {
	store SessionStore
	now   func() time.Time
}

func NewSessionService(store SessionStore) *SessionService {
	return &SessionService{
		store: store,
		now:   time.Now,
	}
}

type ResolveSnapshotInput struct {
	RuntimeID      string
	AgentID        string
	IssueID        string
	ChatSessionID  string
	BaseSnapshotID string
}

type ResumeSelection struct {
	Source         string
	SnapshotID     string
	LastWorkdir    string
	LastBranch     string
	CodexSessionID string
}

func (s *SessionService) ResolveSnapshot(ctx context.Context, in ResolveSnapshotInput) (ResumeSelection, error) {
	if in.IssueID == "" && in.ChatSessionID == "" {
		return ResumeSelection{}, fmt.Errorf("issue_id or chat_session_id is required")
	}

	if in.IssueID != "" {
		session, err := s.store.GetCloudRuntimeSessionForIssue(ctx, db.GetCloudRuntimeSessionForIssueParams{
			RuntimeID: parseUUIDOrZero(in.RuntimeID),
			AgentID:   parseUUIDOrZero(in.AgentID),
			IssueID:   parseUUIDOrZero(in.IssueID),
		})
		if err == nil {
			if session.LastSnapshotID.Valid && snapshotUsable(session.SnapshotExpiresAt, s.now()) {
				return ResumeSelection{
					Source:         "resume",
					SnapshotID:     session.LastSnapshotID.String,
					LastWorkdir:    session.LastWorkdir.String,
					LastBranch:     session.LastBranch.String,
					CodexSessionID: session.LastCodexSessionID.String,
				}, nil
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return ResumeSelection{}, err
		}
	}
	if in.ChatSessionID != "" {
		session, err := s.store.GetCloudRuntimeSessionForChat(ctx, db.GetCloudRuntimeSessionForChatParams{
			RuntimeID:     parseUUIDOrZero(in.RuntimeID),
			AgentID:       parseUUIDOrZero(in.AgentID),
			ChatSessionID: parseUUIDOrZero(in.ChatSessionID),
		})
		if err == nil {
			if session.LastSnapshotID.Valid && snapshotUsable(session.SnapshotExpiresAt, s.now()) {
				return ResumeSelection{
					Source:         "resume",
					SnapshotID:     session.LastSnapshotID.String,
					LastWorkdir:    session.LastWorkdir.String,
					LastBranch:     session.LastBranch.String,
					CodexSessionID: session.LastCodexSessionID.String,
				}, nil
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return ResumeSelection{}, err
		}
	}

	if in.BaseSnapshotID != "" {
		return ResumeSelection{Source: "base", SnapshotID: in.BaseSnapshotID}, nil
	}
	return ResumeSelection{Source: "none"}, nil
}

type RecordSnapshotInput struct {
	RuntimeID      string
	AgentID        string
	IssueID        string
	ChatSessionID  string
	SandboxID      string
	SnapshotID     string
	SnapshotExpiry time.Time
	LastWorkdir    string
	LastBranch     string
	CodexSessionID string
}

func (s *SessionService) RecordSnapshot(ctx context.Context, in RecordSnapshotInput) error {
	if in.IssueID == "" && in.ChatSessionID == "" {
		return fmt.Errorf("issue_id or chat_session_id is required")
	}
	if in.IssueID != "" && in.ChatSessionID != "" {
		return fmt.Errorf("issue_id and chat_session_id are mutually exclusive")
	}

	expiresAt := pgtype.Timestamptz{}
	createdAt := pgtype.Timestamptz{}
	if !in.SnapshotExpiry.IsZero() {
		expiresAt = pgtype.Timestamptz{Time: in.SnapshotExpiry.UTC(), Valid: true}
		createdAt = pgtype.Timestamptz{Time: s.now().UTC(), Valid: true}
	}

	_, err := s.store.UpsertCloudRuntimeSession(ctx, db.UpsertCloudRuntimeSessionParams{
		RuntimeID:          parseUUIDOrZero(in.RuntimeID),
		AgentID:            parseUUIDOrZero(in.AgentID),
		IssueID:            parseUUIDOrZero(in.IssueID),
		ChatSessionID:      parseUUIDOrZero(in.ChatSessionID),
		LastSandboxID:      strText(in.SandboxID),
		LastSnapshotID:     strText(in.SnapshotID),
		SnapshotCreatedAt:  createdAt,
		SnapshotExpiresAt:  expiresAt,
		LastWorkdir:        strText(in.LastWorkdir),
		LastBranch:         strText(in.LastBranch),
		LastCodexSessionID: strText(in.CodexSessionID),
	})
	return err
}

func snapshotUsable(expiresAt pgtype.Timestamptz, now time.Time) bool {
	if !expiresAt.Valid {
		return true
	}
	return expiresAt.Time.After(now)
}

func parseUUIDOrZero(raw string) pgtype.UUID {
	if raw == "" {
		return pgtype.UUID{}
	}
	var v pgtype.UUID
	if err := v.Scan(raw); err != nil {
		return pgtype.UUID{}
	}
	return v
}

func strText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
