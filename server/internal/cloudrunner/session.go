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

type ResumeMode string

const (
	ResumeModeSandbox  ResumeMode = "sandbox"
	ResumeModeSnapshot ResumeMode = "snapshot"
	ResumeModeLocal    ResumeMode = "local"
	ResumeModeNone     ResumeMode = "none"
)

func NewSessionService(store SessionStore) *SessionService {
	return &SessionService{
		store: store,
		now:   time.Now,
	}
}

type ResolveSnapshotInput struct {
	RuntimeID           string
	AgentID             string
	IssueID             string
	ChatSessionID       string
	BaseSnapshotID      string
	PreferredResumeMode ResumeMode
}

type ResumeSelection struct {
	Source            string
	SnapshotID        string
	SnapshotExpiresAt time.Time
	LastWorkdir       string
	LastBranch        string
	CodexSessionID    string
}

func (s *SessionService) ResolveSnapshot(ctx context.Context, in ResolveSnapshotInput) (ResumeSelection, error) {
	if in.IssueID == "" && in.ChatSessionID == "" {
		return ResumeSelection{}, fmt.Errorf("issue_id or chat_session_id is required")
	}

	if in.PreferredResumeMode != "" {
		switch in.PreferredResumeMode {
		case ResumeModeSandbox:
			if selection, ok, err := s.loadPreferredSelection(ctx, in, false, "sandbox"); err != nil {
				return ResumeSelection{}, err
			} else if ok {
				return selection, nil
			}
			return ResumeSelection{Source: string(ResumeModeSandbox)}, nil
		case ResumeModeSnapshot:
			if selection, ok, err := s.loadPreferredSelection(ctx, in, true, "resume"); err != nil {
				return ResumeSelection{}, err
			} else if ok {
				return selection, nil
			}
			return ResumeSelection{}, fmt.Errorf("snapshot resume requested but no usable snapshot is available")
		case ResumeModeLocal:
			return ResumeSelection{Source: string(ResumeModeLocal)}, nil
		default:
			return ResumeSelection{}, fmt.Errorf("unsupported preferred resume mode %q", in.PreferredResumeMode)
		}
	}

	if in.IssueID != "" {
		session, err := s.store.GetCloudRuntimeSessionForIssue(ctx, db.GetCloudRuntimeSessionForIssueParams{
			RuntimeID: parseUUIDOrZero(in.RuntimeID),
			AgentID:   parseUUIDOrZero(in.AgentID),
			IssueID:   parseUUIDOrZero(in.IssueID),
		})
		if err == nil {
			if selection, ok := resumeSelectionFromSession(session, s.now()); ok {
				return selection, nil
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
			if selection, ok := resumeSelectionFromSession(session, s.now()); ok {
				return selection, nil
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

func (s *SessionService) loadPreferredSelection(ctx context.Context, in ResolveSnapshotInput, includeSnapshot bool, source string) (ResumeSelection, bool, error) {
	if in.IssueID != "" {
		session, err := s.store.GetCloudRuntimeSessionForIssue(ctx, db.GetCloudRuntimeSessionForIssueParams{
			RuntimeID: parseUUIDOrZero(in.RuntimeID),
			AgentID:   parseUUIDOrZero(in.AgentID),
			IssueID:   parseUUIDOrZero(in.IssueID),
		})
		if err == nil {
			if selection, ok := preferredSelectionFromSession(session, s.now(), includeSnapshot, source); ok {
				return selection, true, nil
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return ResumeSelection{}, false, err
		}
	}
	if in.ChatSessionID != "" {
		session, err := s.store.GetCloudRuntimeSessionForChat(ctx, db.GetCloudRuntimeSessionForChatParams{
			RuntimeID:     parseUUIDOrZero(in.RuntimeID),
			AgentID:       parseUUIDOrZero(in.AgentID),
			ChatSessionID: parseUUIDOrZero(in.ChatSessionID),
		})
		if err == nil {
			if selection, ok := preferredSelectionFromSession(session, s.now(), includeSnapshot, source); ok {
				return selection, true, nil
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return ResumeSelection{}, false, err
		}
	}
	return ResumeSelection{}, false, nil
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

func (in RecordSnapshotInput) hasPortableResumeState() bool {
	return in.SnapshotID != "" && in.LastWorkdir != "" && in.LastBranch != "" && in.CodexSessionID != ""
}

func (s *SessionService) RecordSnapshot(ctx context.Context, in RecordSnapshotInput) error {
	if err := validateCheckpointInput(in); err != nil {
		return err
	}

	expiresAt := pgtype.Timestamptz{}
	createdAt := pgtype.Timestamptz{}
	if in.SnapshotID != "" {
		createdAt = pgtype.Timestamptz{Time: s.now().UTC(), Valid: true}
	}
	if !in.SnapshotExpiry.IsZero() {
		expiresAt = pgtype.Timestamptz{Time: in.SnapshotExpiry.UTC(), Valid: true}
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

func validateCheckpointInput(in RecordSnapshotInput) error {
	if in.RuntimeID == "" {
		return fmt.Errorf("runtime_id is required")
	}
	if in.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if in.IssueID == "" && in.ChatSessionID == "" {
		return fmt.Errorf("issue_id or chat_session_id is required")
	}
	if in.IssueID != "" && in.ChatSessionID != "" {
		return fmt.Errorf("issue_id and chat_session_id are mutually exclusive")
	}
	if in.SnapshotID == "" {
		return nil
	}
	if in.LastWorkdir == "" {
		return fmt.Errorf("last_workdir is required when snapshot_id is set")
	}
	if in.LastBranch == "" {
		return fmt.Errorf("last_branch is required when snapshot_id is set")
	}
	if in.CodexSessionID == "" {
		return fmt.Errorf("last_codex_session_id is required when snapshot_id is set")
	}
	return nil
}

func resumeSelectionFromSession(session db.CloudRuntimeSession, now time.Time) (ResumeSelection, bool) {
	return preferredSelectionFromSession(session, now, true, "resume")
}

func preferredSelectionFromSession(session db.CloudRuntimeSession, now time.Time, includeSnapshot bool, source string) (ResumeSelection, bool) {
	if !session.LastWorkdir.Valid || !session.LastBranch.Valid || !session.LastCodexSessionID.Valid {
		return ResumeSelection{}, false
	}
	selection := ResumeSelection{
		Source:         source,
		LastWorkdir:    session.LastWorkdir.String,
		LastBranch:     session.LastBranch.String,
		CodexSessionID: session.LastCodexSessionID.String,
	}
	if includeSnapshot {
		if !session.LastSnapshotID.Valid || !snapshotUsable(session.SnapshotExpiresAt, now) {
			return ResumeSelection{}, false
		}
		selection.SnapshotID = session.LastSnapshotID.String
		if session.SnapshotExpiresAt.Valid {
			selection.SnapshotExpiresAt = session.SnapshotExpiresAt.Time
		}
	}
	return selection, true
}

func resumeSelectionFromSessionLegacy(session db.CloudRuntimeSession, now time.Time) (ResumeSelection, bool) {
	if !session.LastSnapshotID.Valid || !snapshotUsable(session.SnapshotExpiresAt, now) {
		return ResumeSelection{}, false
	}
	if !session.LastWorkdir.Valid || !session.LastBranch.Valid || !session.LastCodexSessionID.Valid {
		return ResumeSelection{}, false
	}

	selection := ResumeSelection{
		Source:         "resume",
		SnapshotID:     session.LastSnapshotID.String,
		LastWorkdir:    session.LastWorkdir.String,
		LastBranch:     session.LastBranch.String,
		CodexSessionID: session.LastCodexSessionID.String,
	}
	if session.SnapshotExpiresAt.Valid {
		selection.SnapshotExpiresAt = session.SnapshotExpiresAt.Time
	}
	return selection, true
}

func snapshotUsable(expiresAt pgtype.Timestamptz, now time.Time) bool {
	if !expiresAt.Valid {
		return true
	}
	return expiresAt.Time.After(now)
}

func CanResumeFromSandbox(sandboxHealthy, budgetAllowed bool) bool {
	return sandboxHealthy && budgetAllowed
}

func SelectResumeMode(sandboxHealthy, budgetAllowed, hasSnapshot, localHandoffAvailable bool) ResumeMode {
	switch {
	case CanResumeFromSandbox(sandboxHealthy, budgetAllowed):
		return ResumeModeSandbox
	case hasSnapshot:
		return ResumeModeSnapshot
	case localHandoffAvailable:
		return ResumeModeLocal
	default:
		return ResumeModeNone
	}
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
