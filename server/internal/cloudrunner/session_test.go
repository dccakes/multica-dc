package cloudrunner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type fakeSessionStore struct {
	getIssueFn func(ctx context.Context, arg db.GetCloudRuntimeSessionForIssueParams) (db.CloudRuntimeSession, error)
	getChatFn  func(ctx context.Context, arg db.GetCloudRuntimeSessionForChatParams) (db.CloudRuntimeSession, error)
	upsertFn   func(ctx context.Context, arg db.UpsertCloudRuntimeSessionParams) (db.UpsertCloudRuntimeSessionRow, error)
	lastUpsert db.UpsertCloudRuntimeSessionParams
}

func (f *fakeSessionStore) GetCloudRuntimeSessionForIssue(ctx context.Context, arg db.GetCloudRuntimeSessionForIssueParams) (db.CloudRuntimeSession, error) {
	if f.getIssueFn != nil {
		return f.getIssueFn(ctx, arg)
	}
	return db.CloudRuntimeSession{}, pgx.ErrNoRows
}

func (f *fakeSessionStore) UpsertCloudRuntimeSession(ctx context.Context, arg db.UpsertCloudRuntimeSessionParams) (db.UpsertCloudRuntimeSessionRow, error) {
	f.lastUpsert = arg
	if f.upsertFn != nil {
		return f.upsertFn(ctx, arg)
	}
	return db.UpsertCloudRuntimeSessionRow{}, nil
}

func (f *fakeSessionStore) GetCloudRuntimeSessionForChat(ctx context.Context, arg db.GetCloudRuntimeSessionForChatParams) (db.CloudRuntimeSession, error) {
	if f.getChatFn != nil {
		return f.getChatFn(ctx, arg)
	}
	return db.CloudRuntimeSession{}, pgx.ErrNoRows
}

func mustUUID(t *testing.T, raw string) pgtype.UUID {
	t.Helper()
	var v pgtype.UUID
	if err := v.Scan(raw); err != nil {
		t.Fatalf("invalid uuid %q: %v", raw, err)
	}
	return v
}

func TestResolveSnapshot_UsesResumeSnapshotWhenUnexpired(t *testing.T) {
	expiry := time.Now().UTC().Add(2 * time.Hour)
	store := &fakeSessionStore{
		getIssueFn: func(_ context.Context, _ db.GetCloudRuntimeSessionForIssueParams) (db.CloudRuntimeSession, error) {
			return db.CloudRuntimeSession{
				LastSnapshotID:     pgtype.Text{String: "snap_resume", Valid: true},
				SnapshotExpiresAt:  pgtype.Timestamptz{Time: expiry, Valid: true},
				LastWorkdir:        pgtype.Text{String: "/workspace", Valid: true},
				LastBranch:         pgtype.Text{String: "feature/a", Valid: true},
				LastCodexSessionID: pgtype.Text{String: "codex_123", Valid: true},
			}, nil
		},
	}
	svc := NewSessionService(store)
	svc.now = func() time.Time { return time.Now().UTC() }

	selection, err := svc.ResolveSnapshot(context.Background(), ResolveSnapshotInput{
		RuntimeID:      "11111111-1111-1111-1111-111111111111",
		AgentID:        "22222222-2222-2222-2222-222222222222",
		IssueID:        "33333333-3333-3333-3333-333333333333",
		BaseSnapshotID: "snap_base",
	})
	if err != nil {
		t.Fatalf("ResolveSnapshot() error = %v", err)
	}
	if selection.Source != "resume" || selection.SnapshotID != "snap_resume" {
		t.Fatalf("selection = %#v, want resume snapshot", selection)
	}
	if !selection.SnapshotExpiresAt.Equal(expiry) {
		t.Fatalf("selection.SnapshotExpiresAt = %v, want %v", selection.SnapshotExpiresAt, expiry)
	}
}

func TestResolveSnapshot_FallsBackToBaseWhenResumeExpired(t *testing.T) {
	store := &fakeSessionStore{
		getIssueFn: func(_ context.Context, _ db.GetCloudRuntimeSessionForIssueParams) (db.CloudRuntimeSession, error) {
			return db.CloudRuntimeSession{
				LastSnapshotID:    pgtype.Text{String: "snap_old", Valid: true},
				SnapshotExpiresAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-1 * time.Minute), Valid: true},
			}, nil
		},
	}
	svc := NewSessionService(store)
	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	selection, err := svc.ResolveSnapshot(context.Background(), ResolveSnapshotInput{
		RuntimeID:      "11111111-1111-1111-1111-111111111111",
		AgentID:        "22222222-2222-2222-2222-222222222222",
		IssueID:        "33333333-3333-3333-3333-333333333333",
		BaseSnapshotID: "snap_base",
	})
	if err != nil {
		t.Fatalf("ResolveSnapshot() error = %v", err)
	}
	if selection.Source != "base" || selection.SnapshotID != "snap_base" {
		t.Fatalf("selection = %#v, want base snapshot", selection)
	}
}

func TestResolveSnapshot_FallsBackToChatWhenIssueSnapshotExpired(t *testing.T) {
	issueExpiry := time.Now().UTC().Add(-1 * time.Minute)
	chatExpiry := time.Now().UTC().Add(2 * time.Hour)
	store := &fakeSessionStore{
		getIssueFn: func(_ context.Context, _ db.GetCloudRuntimeSessionForIssueParams) (db.CloudRuntimeSession, error) {
			return db.CloudRuntimeSession{
				LastSnapshotID:    pgtype.Text{String: "snap_issue_old", Valid: true},
				SnapshotExpiresAt: pgtype.Timestamptz{Time: issueExpiry, Valid: true},
			}, nil
		},
		getChatFn: func(_ context.Context, _ db.GetCloudRuntimeSessionForChatParams) (db.CloudRuntimeSession, error) {
			return db.CloudRuntimeSession{
				LastSnapshotID:    pgtype.Text{String: "snap_chat", Valid: true},
				SnapshotExpiresAt: pgtype.Timestamptz{Time: chatExpiry, Valid: true},
				LastWorkdir:       pgtype.Text{String: "/chat-workspace", Valid: true},
				LastBranch:        pgtype.Text{String: "chat/branch", Valid: true},
				LastCodexSessionID: pgtype.Text{
					String: "chat_codex",
					Valid:  true,
				},
			}, nil
		},
	}
	svc := NewSessionService(store)
	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	selection, err := svc.ResolveSnapshot(context.Background(), ResolveSnapshotInput{
		RuntimeID:     "11111111-1111-1111-1111-111111111111",
		AgentID:       "22222222-2222-2222-2222-222222222222",
		IssueID:       "33333333-3333-3333-3333-333333333333",
		ChatSessionID: "44444444-4444-4444-4444-444444444444",
	})
	if err != nil {
		t.Fatalf("ResolveSnapshot() error = %v", err)
	}
	if selection.Source != "resume" || selection.SnapshotID != "snap_chat" {
		t.Fatalf("selection = %#v, want chat resume snapshot", selection)
	}
	if !selection.SnapshotExpiresAt.Equal(chatExpiry) {
		t.Fatalf("selection.SnapshotExpiresAt = %v, want %v", selection.SnapshotExpiresAt, chatExpiry)
	}
}

func TestResolveSnapshot_PropagatesStoreError(t *testing.T) {
	store := &fakeSessionStore{
		getIssueFn: func(_ context.Context, _ db.GetCloudRuntimeSessionForIssueParams) (db.CloudRuntimeSession, error) {
			return db.CloudRuntimeSession{}, errors.New("db failed")
		},
	}
	svc := NewSessionService(store)

	_, err := svc.ResolveSnapshot(context.Background(), ResolveSnapshotInput{
		RuntimeID: "11111111-1111-1111-1111-111111111111",
		AgentID:   "22222222-2222-2222-2222-222222222222",
		IssueID:   "33333333-3333-3333-3333-333333333333",
	})
	if err == nil {
		t.Fatal("ResolveSnapshot() error = nil, want store error")
	}
}

func TestResolveSnapshot_UsesChatResumeSnapshotWhenUnexpired(t *testing.T) {
	expiry := time.Now().UTC().Add(2 * time.Hour)
	store := &fakeSessionStore{
		getChatFn: func(_ context.Context, _ db.GetCloudRuntimeSessionForChatParams) (db.CloudRuntimeSession, error) {
			return db.CloudRuntimeSession{
				LastSnapshotID:     pgtype.Text{String: "snap_chat", Valid: true},
				SnapshotExpiresAt:  pgtype.Timestamptz{Time: expiry, Valid: true},
				LastWorkdir:        pgtype.Text{String: "/chat-workspace", Valid: true},
				LastBranch:         pgtype.Text{String: "chat/branch", Valid: true},
				LastCodexSessionID: pgtype.Text{String: "chat_codex", Valid: true},
			}, nil
		},
	}
	svc := NewSessionService(store)

	selection, err := svc.ResolveSnapshot(context.Background(), ResolveSnapshotInput{
		RuntimeID:     "11111111-1111-1111-1111-111111111111",
		AgentID:       "22222222-2222-2222-2222-222222222222",
		ChatSessionID: "44444444-4444-4444-4444-444444444444",
	})
	if err != nil {
		t.Fatalf("ResolveSnapshot() error = %v", err)
	}
	if selection.Source != "resume" || selection.SnapshotID != "snap_chat" {
		t.Fatalf("selection = %#v, want chat resume snapshot", selection)
	}
	if !selection.SnapshotExpiresAt.Equal(expiry) {
		t.Fatalf("selection.SnapshotExpiresAt = %v, want %v", selection.SnapshotExpiresAt, expiry)
	}
}

func TestRecordSnapshot_UpsertsSession(t *testing.T) {
	store := &fakeSessionStore{}
	svc := NewSessionService(store)
	fixedNow := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }

	expiry := fixedNow.Add(24 * time.Hour)
	err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		RuntimeID:      "11111111-1111-1111-1111-111111111111",
		AgentID:        "22222222-2222-2222-2222-222222222222",
		IssueID:        "33333333-3333-3333-3333-333333333333",
		SandboxID:      "sandbox_1",
		SnapshotID:     "snapshot_1",
		SnapshotExpiry: expiry,
		LastWorkdir:    "/workspace",
		LastBranch:     "main",
		CodexSessionID: "codex_session_1",
	})
	if err != nil {
		t.Fatalf("RecordSnapshot() error = %v", err)
	}

	if store.lastUpsert.RuntimeID != mustUUID(t, "11111111-1111-1111-1111-111111111111") {
		t.Fatalf("RuntimeID upsert mismatch")
	}
	if store.lastUpsert.IssueID != mustUUID(t, "33333333-3333-3333-3333-333333333333") {
		t.Fatalf("IssueID upsert mismatch")
	}
	if store.lastUpsert.LastSnapshotID.String != "snapshot_1" {
		t.Fatalf("LastSnapshotID = %q, want snapshot_1", store.lastUpsert.LastSnapshotID.String)
	}
	if !store.lastUpsert.SnapshotExpiresAt.Valid || !store.lastUpsert.SnapshotExpiresAt.Time.Equal(expiry) {
		t.Fatalf("SnapshotExpiresAt = %#v, want %v", store.lastUpsert.SnapshotExpiresAt, expiry)
	}
	if !store.lastUpsert.SnapshotCreatedAt.Valid || !store.lastUpsert.SnapshotCreatedAt.Time.Equal(fixedNow) {
		t.Fatalf("SnapshotCreatedAt = %#v, want %v", store.lastUpsert.SnapshotCreatedAt, fixedNow)
	}
}

func TestRecordSnapshot_RejectsIncompletePortableCheckpoint(t *testing.T) {
	store := &fakeSessionStore{}
	svc := NewSessionService(store)

	err := svc.RecordSnapshot(context.Background(), RecordSnapshotInput{
		RuntimeID:      "11111111-1111-1111-1111-111111111111",
		AgentID:        "22222222-2222-2222-2222-222222222222",
		IssueID:        "33333333-3333-3333-3333-333333333333",
		SnapshotID:     "snapshot_1",
		LastWorkdir:    "",
		LastBranch:     "main",
		CodexSessionID: "codex_session_1",
	})
	if err == nil {
		t.Fatal("RecordSnapshot() error = nil, want portable checkpoint validation failure")
	}
	if store.lastUpsert != (db.UpsertCloudRuntimeSessionParams{}) {
		t.Fatalf("RecordSnapshot() should not upsert on invalid checkpoint, got %#v", store.lastUpsert)
	}
}
