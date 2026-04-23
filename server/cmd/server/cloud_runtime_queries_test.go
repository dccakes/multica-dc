package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func mustUUID(t *testing.T, raw string) pgtype.UUID {
	t.Helper()
	var v pgtype.UUID
	if err := v.Scan(raw); err != nil {
		t.Fatalf("invalid uuid %q: %v", raw, err)
	}
	return v
}

func mustText(t *testing.T, raw string) pgtype.Text {
	t.Helper()
	return pgtype.Text{String: raw, Valid: true}
}

func TestCloudRuntimeCredentialCRUDQueries(t *testing.T) {
	ctx := context.Background()
	q := db.New(testPool)

	workspaceID := mustUUID(t, testWorkspaceID)
	ownerID := mustUUID(t, testUserID)

	created, err := q.CreateCloudRuntimeCredential(ctx, db.CreateCloudRuntimeCredentialParams{
		WorkspaceID:    workspaceID,
		Name:           "Vercel Primary",
		Provider:       "vercel_sandbox",
		EncryptedToken: "encrypted-token-v1",
		ProjectID:      "prj_123",
		TeamID:         mustText(t, "team_123"),
		BaseSnapshotID: mustText(t, "snap_base"),
		Region:         "iad1",
		Status:         "active",
		OwnerID:        ownerID,
	})
	if err != nil {
		t.Fatalf("CreateCloudRuntimeCredential failed: %v", err)
	}
	t.Cleanup(func() {
		_ = q.DeleteCloudRuntimeCredential(context.Background(), db.DeleteCloudRuntimeCredentialParams{
			ID:          created.ID,
			WorkspaceID: workspaceID,
		})
	})

	if created.Name != "Vercel Primary" {
		t.Fatalf("expected created name to be Vercel Primary, got %q", created.Name)
	}

	got, err := q.GetCloudRuntimeCredential(ctx, db.GetCloudRuntimeCredentialParams{
		ID:          created.ID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		t.Fatalf("GetCloudRuntimeCredential failed: %v", err)
	}
	if got.EncryptedToken != "encrypted-token-v1" {
		t.Fatalf("expected encrypted token to round-trip")
	}

	list, err := q.ListCloudRuntimeCredentials(ctx, workspaceID)
	if err != nil {
		t.Fatalf("ListCloudRuntimeCredentials failed: %v", err)
	}
	found := false
	for _, cred := range list {
		if cred.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected created credential to be listed")
	}

	testedAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	updated, err := q.UpdateCloudRuntimeCredential(ctx, db.UpdateCloudRuntimeCredentialParams{
		ID:            created.ID,
		WorkspaceID:   workspaceID,
		Name:          "Vercel Updated",
		ProjectID:     "prj_456",
		TeamID:        mustText(t, "team_456"),
		BaseSnapshotID: mustText(t, "snap_updated"),
		Region:        "sfo1",
		Status:        "active",
		LastTestedAt:  testedAt,
		LastTestError: pgtype.Text{},
	})
	if err != nil {
		t.Fatalf("UpdateCloudRuntimeCredential failed: %v", err)
	}
	if updated.Name != "Vercel Updated" || updated.ProjectID != "prj_456" {
		t.Fatalf("expected updated fields to persist")
	}

	if err := q.DeleteCloudRuntimeCredential(ctx, db.DeleteCloudRuntimeCredentialParams{
		ID:          created.ID,
		WorkspaceID: workspaceID,
	}); err != nil {
		t.Fatalf("DeleteCloudRuntimeCredential failed: %v", err)
	}

	if _, err := q.GetCloudRuntimeCredential(ctx, db.GetCloudRuntimeCredentialParams{
		ID:          created.ID,
		WorkspaceID: workspaceID,
	}); err == nil {
		t.Fatalf("expected deleted credential lookup to fail")
	}
}

func TestCloudRuntimeSessionUpsertAndGetForIssue(t *testing.T) {
	ctx := context.Background()
	q := db.New(testPool)

	workspaceID := mustUUID(t, testWorkspaceID)
	ownerID := mustUUID(t, testUserID)

	credential, err := q.CreateCloudRuntimeCredential(ctx, db.CreateCloudRuntimeCredentialParams{
		WorkspaceID:    workspaceID,
		Name:           "Runtime Session Credential",
		Provider:       "vercel_sandbox",
		EncryptedToken: "encrypted-session-token",
		ProjectID:      "prj_session",
		TeamID:         pgtype.Text{},
		BaseSnapshotID: pgtype.Text{},
		Region:         "iad1",
		Status:         "active",
		OwnerID:        ownerID,
	})
	if err != nil {
		t.Fatalf("CreateCloudRuntimeCredential failed: %v", err)
	}
	t.Cleanup(func() {
		_ = q.DeleteCloudRuntimeCredential(context.Background(), db.DeleteCloudRuntimeCredentialParams{
			ID:          credential.ID,
			WorkspaceID: workspaceID,
		})
	})

	var runtimeID pgtype.UUID
	if err := testPool.QueryRow(ctx, `
		SELECT id FROM agent_runtime
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, workspaceID).Scan(&runtimeID); err != nil {
		t.Fatalf("failed to load runtime id: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		UPDATE agent_runtime
		SET credential_id = $2
		WHERE id = $1
	`, runtimeID, credential.ID); err != nil {
		t.Fatalf("failed to bind runtime to credential: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `UPDATE agent_runtime SET credential_id = NULL WHERE id = $1`, runtimeID)
	})

	var agentID pgtype.UUID
	if err := testPool.QueryRow(ctx, `
		SELECT id FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, workspaceID).Scan(&agentID); err != nil {
		t.Fatalf("failed to load agent id: %v", err)
	}

	var issueID pgtype.UUID
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, position, creator_type, creator_id)
		VALUES ($1, 'Cloud runtime issue', 'todo', 'medium', 0, 'member', gen_random_uuid())
		RETURNING id
	`, workspaceID).Scan(&issueID); err != nil {
		t.Fatalf("failed to create issue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issueID)
	})

	first, err := q.UpsertCloudRuntimeSession(ctx, db.UpsertCloudRuntimeSessionParams{
		RuntimeID:           runtimeID,
		AgentID:             agentID,
		IssueID:             issueID,
		ChatSessionID:       pgtype.UUID{},
		LastSandboxID:       mustText(t, "sandbox_1"),
		LastSnapshotID:      mustText(t, "snapshot_1"),
		SnapshotCreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		SnapshotExpiresAt:   pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
		LastWorkdir:         mustText(t, "/workspace"),
		LastBranch:          mustText(t, "main"),
		LastCodexSessionID:  mustText(t, "codex_1"),
	})
	if err != nil {
		t.Fatalf("UpsertCloudRuntimeSession (insert) failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM cloud_runtime_session WHERE id = $1`, first.ID)
	})

	second, err := q.UpsertCloudRuntimeSession(ctx, db.UpsertCloudRuntimeSessionParams{
		RuntimeID:           runtimeID,
		AgentID:             agentID,
		IssueID:             issueID,
		ChatSessionID:       pgtype.UUID{},
		LastSandboxID:       mustText(t, "sandbox_2"),
		LastSnapshotID:      mustText(t, "snapshot_2"),
		SnapshotCreatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		SnapshotExpiresAt:   pgtype.Timestamptz{Time: time.Now().UTC().Add(48 * time.Hour), Valid: true},
		LastWorkdir:         mustText(t, "/workspace-2"),
		LastBranch:          mustText(t, "feature/cloud"),
		LastCodexSessionID:  mustText(t, "codex_2"),
	})
	if err != nil {
		t.Fatalf("UpsertCloudRuntimeSession (update) failed: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected upsert to update the same row")
	}

	got, err := q.GetCloudRuntimeSessionForIssue(ctx, db.GetCloudRuntimeSessionForIssueParams{
		RuntimeID: runtimeID,
		AgentID:   agentID,
		IssueID:   issueID,
	})
	if err != nil {
		t.Fatalf("GetCloudRuntimeSessionForIssue failed: %v", err)
	}
	if got.LastSandboxID.String != "sandbox_2" || got.LastSnapshotID.String != "snapshot_2" {
		t.Fatalf("expected latest session values after upsert")
	}
}

func TestCloudRuntimeSessionGetForChat(t *testing.T) {
	ctx := context.Background()
	q := db.New(testPool)

	workspaceID := mustUUID(t, testWorkspaceID)

	var runtimeID pgtype.UUID
	if err := testPool.QueryRow(ctx, `
		SELECT id FROM agent_runtime
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, workspaceID).Scan(&runtimeID); err != nil {
		t.Fatalf("failed to load runtime id: %v", err)
	}

	var agentID pgtype.UUID
	if err := testPool.QueryRow(ctx, `
		SELECT id FROM agent
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, workspaceID).Scan(&agentID); err != nil {
		t.Fatalf("failed to load agent id: %v", err)
	}

	var chatSessionID pgtype.UUID
	if err := testPool.QueryRow(ctx, `
		INSERT INTO chat_session (workspace_id, agent_id, creator_id, title, status)
		VALUES ($1, $2, $3, 'Cloud runtime chat', 'active')
		RETURNING id
	`, workspaceID, agentID, mustUUID(t, testUserID)).Scan(&chatSessionID); err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, chatSessionID)
	})

	row, err := q.UpsertCloudRuntimeSession(ctx, db.UpsertCloudRuntimeSessionParams{
		RuntimeID:          runtimeID,
		AgentID:            agentID,
		IssueID:            pgtype.UUID{},
		ChatSessionID:      chatSessionID,
		LastSandboxID:      mustText(t, "sandbox_chat"),
		LastSnapshotID:     mustText(t, "snapshot_chat"),
		SnapshotCreatedAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		SnapshotExpiresAt:  pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
		LastWorkdir:        mustText(t, "/workspace-chat"),
		LastBranch:         mustText(t, "main"),
		LastCodexSessionID: mustText(t, "codex_chat"),
	})
	if err != nil {
		t.Fatalf("UpsertCloudRuntimeSession (chat) failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM cloud_runtime_session WHERE id = $1`, row.ID)
	})

	got, err := q.GetCloudRuntimeSessionForChat(ctx, db.GetCloudRuntimeSessionForChatParams{
		RuntimeID:     runtimeID,
		AgentID:       agentID,
		ChatSessionID: chatSessionID,
	})
	if err != nil {
		t.Fatalf("GetCloudRuntimeSessionForChat failed: %v", err)
	}
	if got.LastSnapshotID.String != "snapshot_chat" {
		t.Fatalf("expected snapshot_chat, got %q", got.LastSnapshotID.String)
	}
}
