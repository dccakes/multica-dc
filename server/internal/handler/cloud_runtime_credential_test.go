package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestVercelCloudRuntimeCredentialCRUD(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	unique := fmt.Sprintf("%d", time.Now().UnixNano())
	createName := "Vercel CRUD " + unique
	createProjectID := "prj_" + unique

	createBody := map[string]any{
		"name":             createName,
		"token":            "vercel-token-" + unique,
		"project_id":       createProjectID,
		"team_id":          "team_" + unique,
		"base_snapshot_id": "snap_" + unique,
		"region":           "iad1",
	}

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/cloud/credentials/vercel", createBody)
	testHandler.CreateVercelCloudRuntimeCredential(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateVercelCloudRuntimeCredential: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created CloudRuntimeCredentialResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created id")
	}
	if created.Provider != "vercel_sandbox" {
		t.Fatalf("expected provider vercel_sandbox, got %q", created.Provider)
	}
	if created.Name != createName {
		t.Fatalf("expected name %q, got %q", createName, created.Name)
	}
	if !created.HasToken {
		t.Fatalf("expected has_token true")
	}

	credentialID := created.ID
	t.Cleanup(func() {
		_ = testHandler.Queries.DeleteCloudRuntimeCredential(context.Background(), db.DeleteCloudRuntimeCredentialParams{
			ID:          parseUUID(credentialID),
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})

	w = httptest.NewRecorder()
	req = newRequest(http.MethodGet, "/api/cloud/credentials/vercel", nil)
	testHandler.ListVercelCloudRuntimeCredentials(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListVercelCloudRuntimeCredentials: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var listResp []CloudRuntimeCredentialResponse
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	found := false
	for _, c := range listResp {
		if c.ID == credentialID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected created credential %s in list", credentialID)
	}

	updateBody := map[string]any{
		"name":             "Vercel CRUD Updated " + unique,
		"project_id":       "prj_updated_" + unique,
		"team_id":          "team_updated_" + unique,
		"base_snapshot_id": "snap_updated_" + unique,
		"region":           "sfo1",
		"status":           "inactive",
	}

	w = httptest.NewRecorder()
	req = newRequest(http.MethodPatch, "/api/cloud/credentials/vercel/"+credentialID, updateBody)
	req = withURLParam(req, "id", credentialID)
	testHandler.UpdateVercelCloudRuntimeCredential(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateVercelCloudRuntimeCredential: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var updated CloudRuntimeCredentialResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	expectedUpdatedName := updateBody["name"].(string)
	expectedUpdatedProjectID := updateBody["project_id"].(string)
	expectedUpdatedRegion := updateBody["region"].(string)
	expectedUpdatedStatus := updateBody["status"].(string)
	if updated.Name != expectedUpdatedName {
		t.Fatalf("expected updated name %q, got %q", expectedUpdatedName, updated.Name)
	}
	if updated.ProjectID != expectedUpdatedProjectID {
		t.Fatalf("expected updated project_id %q, got %q", expectedUpdatedProjectID, updated.ProjectID)
	}
	if updated.Region != expectedUpdatedRegion {
		t.Fatalf("expected updated region %q, got %q", expectedUpdatedRegion, updated.Region)
	}
	if updated.Status != expectedUpdatedStatus {
		t.Fatalf("expected updated status %q, got %q", expectedUpdatedStatus, updated.Status)
	}

	w = httptest.NewRecorder()
	req = newRequest(http.MethodDelete, "/api/cloud/credentials/vercel/"+credentialID, nil)
	req = withURLParam(req, "id", credentialID)
	testHandler.DeleteVercelCloudRuntimeCredential(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteVercelCloudRuntimeCredential: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newRequest(http.MethodGet, "/api/cloud/credentials/vercel", nil)
	testHandler.ListVercelCloudRuntimeCredentials(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListVercelCloudRuntimeCredentials after delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	listResp = nil
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list after delete response: %v", err)
	}
	for _, c := range listResp {
		if c.ID == credentialID {
			t.Fatalf("expected credential %s to be deleted", credentialID)
		}
	}
}

func TestVercelCloudRuntimeCredentialTestAndBootstrap(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	cred := createVercelCredentialForHandlerTest(t, fmt.Sprintf("stub-%d", time.Now().UnixNano()))

	w := httptest.NewRecorder()
	req := newRequest(http.MethodPost, "/api/cloud/credentials/vercel/"+cred.ID+"/test", nil)
	req = withURLParam(req, "id", cred.ID)
	testHandler.TestVercelCloudRuntimeCredential(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("TestVercelCloudRuntimeCredential: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var testResp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&testResp); err != nil {
		t.Fatalf("decode test response: %v", err)
	}
	if testResp["credential_id"] != cred.ID {
		t.Fatalf("expected test credential_id %q, got %v", cred.ID, testResp["credential_id"])
	}
	if testResp["ok"] != true {
		t.Fatalf("expected test ok=true, got %v", testResp["ok"])
	}
	if testResp["status"] != "passed" {
		t.Fatalf("expected test status=passed, got %v", testResp["status"])
	}
	if testResp["provider"] != "vercel" {
		t.Fatalf("expected test provider=vercel, got %v", testResp["provider"])
	}
	if _, ok := testResp["checked_at"].(string); !ok {
		t.Fatalf("expected checked_at string, got %T", testResp["checked_at"])
	}

	persisted, err := testHandler.Queries.GetCloudRuntimeCredential(context.Background(), db.GetCloudRuntimeCredentialParams{
		ID:          parseUUID(cred.ID),
		WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load credential after test endpoint: %v", err)
	}
	if !persisted.LastTestedAt.Valid {
		t.Fatalf("expected last_tested_at to be set")
	}
	if persisted.LastTestError.Valid {
		t.Fatalf("expected last_test_error to be cleared, got %q", persisted.LastTestError.String)
	}

	w = httptest.NewRecorder()
	req = newRequest(http.MethodPost, "/api/cloud/credentials/vercel/"+cred.ID+"/bootstrap", nil)
	req = withURLParam(req, "id", cred.ID)
	testHandler.BootstrapVercelCloudRuntimeCredential(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("BootstrapVercelCloudRuntimeCredential: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var bootstrapResp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&bootstrapResp); err != nil {
		t.Fatalf("decode bootstrap response: %v", err)
	}
	if bootstrapResp["credential_id"] != cred.ID {
		t.Fatalf("expected bootstrap credential_id %q, got %v", cred.ID, bootstrapResp["credential_id"])
	}
	if bootstrapResp["status"] != "ready" {
		t.Fatalf("expected bootstrap status=ready, got %v", bootstrapResp["status"])
	}
	if bootstrapResp["provider"] != "vercel" {
		t.Fatalf("expected bootstrap provider=vercel, got %v", bootstrapResp["provider"])
	}
	if bootstrapResp["project_id"] != cred.ProjectID {
		t.Fatalf("expected bootstrap project_id %q, got %v", cred.ProjectID, bootstrapResp["project_id"])
	}
	runtimeID, ok := bootstrapResp["runtime_id"].(string)
	if !ok || runtimeID == "" {
		t.Fatalf("expected bootstrap runtime_id string, got %T (%v)", bootstrapResp["runtime_id"], bootstrapResp["runtime_id"])
	}
	t.Cleanup(func() {
		_ = testHandler.Queries.DeleteAgentRuntime(context.Background(), parseUUID(runtimeID))
	})
	rt, err := testHandler.Queries.GetAgentRuntime(context.Background(), parseUUID(runtimeID))
	if err != nil {
		t.Fatalf("expected bootstrapped runtime to exist: %v", err)
	}
	if !rt.CredentialID.Valid || uuidToString(rt.CredentialID) != cred.ID {
		t.Fatalf("expected runtime credential_id %q, got %q", cred.ID, uuidToString(rt.CredentialID))
	}
	if _, ok := bootstrapResp["command"].(string); !ok {
		t.Fatalf("expected bootstrap command string, got %T", bootstrapResp["command"])
	}
}

func TestVercelCloudRuntimeCredentialRoleEnforcement(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	memberUserID := createWorkspaceUserWithRole(t, "member")
	cred := createVercelCredentialForHandlerTest(t, fmt.Sprintf("role-%d", time.Now().UnixNano()))

	tests := []struct {
		name       string
		method     string
		path       string
		idParam    string
		body       any
		call       func(http.ResponseWriter, *http.Request)
		expectCode int
	}{
		{
			name:       "list denied for member",
			method:     http.MethodGet,
			path:       "/api/cloud/credentials/vercel",
			call:       testHandler.ListVercelCloudRuntimeCredentials,
			expectCode: http.StatusForbidden,
		},
		{
			name:   "create denied for member",
			method: http.MethodPost,
			path:   "/api/cloud/credentials/vercel",
			body: map[string]any{
				"name":       "member-create-denied",
				"token":      "token",
				"project_id": "project",
				"region":     "iad1",
			},
			call:       testHandler.CreateVercelCloudRuntimeCredential,
			expectCode: http.StatusForbidden,
		},
		{
			name:   "update denied for member",
			method: http.MethodPatch,
			path:   "/api/cloud/credentials/vercel/" + cred.ID,
			idParam: cred.ID,
			body: map[string]any{
				"name":       "member-update-denied",
				"project_id": "project",
				"region":     "iad1",
				"status":     "active",
			},
			call:       testHandler.UpdateVercelCloudRuntimeCredential,
			expectCode: http.StatusForbidden,
		},
		{
			name:       "test denied for member",
			method:     http.MethodPost,
			path:       "/api/cloud/credentials/vercel/" + cred.ID + "/test",
			idParam:    cred.ID,
			call:       testHandler.TestVercelCloudRuntimeCredential,
			expectCode: http.StatusForbidden,
		},
		{
			name:       "bootstrap denied for member",
			method:     http.MethodPost,
			path:       "/api/cloud/credentials/vercel/" + cred.ID + "/bootstrap",
			idParam:    cred.ID,
			call:       testHandler.BootstrapVercelCloudRuntimeCredential,
			expectCode: http.StatusForbidden,
		},
		{
			name:       "delete denied for member",
			method:     http.MethodDelete,
			path:       "/api/cloud/credentials/vercel/" + cred.ID,
			idParam:    cred.ID,
			call:       testHandler.DeleteVercelCloudRuntimeCredential,
			expectCode: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := newRequest(tc.method, tc.path, tc.body)
			req.Header.Set("X-User-ID", memberUserID)
			if tc.idParam != "" {
				req = withURLParam(req, "id", tc.idParam)
			}
			tc.call(w, req)
			if w.Code != tc.expectCode {
				t.Fatalf("expected %d, got %d: %s", tc.expectCode, w.Code, w.Body.String())
			}
		})
	}

	adminUserID := createWorkspaceUserWithRole(t, "admin")
	w := httptest.NewRecorder()
	req := newRequest(http.MethodGet, "/api/cloud/credentials/vercel", nil)
	req.Header.Set("X-User-ID", adminUserID)
	testHandler.ListVercelCloudRuntimeCredentials(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected admin list access 200, got %d: %s", w.Code, w.Body.String())
	}
}

func createWorkspaceUserWithRole(t *testing.T, role string) string {
	t.Helper()

	ctx := context.Background()
	unique := time.Now().UnixNano()
	email := fmt.Sprintf("cloud-credential-%s-%d@multica.ai", role, unique)
	name := fmt.Sprintf("Cloud Credential %s %d", role, unique)

	var userID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO "user" (name, email)
		VALUES ($1, $2)
		RETURNING id
	`, name, email).Scan(&userID); err != nil {
		t.Fatalf("create user with role %s: %v", role, err)
	}

	if _, err := testPool.Exec(ctx, `
		INSERT INTO member (workspace_id, user_id, role)
		VALUES ($1, $2, $3)
	`, testWorkspaceID, userID, role); err != nil {
		t.Fatalf("add user to workspace with role %s: %v", role, err)
	}

	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID)
	})

	return userID
}

type credentialFixture struct {
	ID        string
	ProjectID string
}

func createVercelCredentialForHandlerTest(t *testing.T, name string) credentialFixture {
	t.Helper()

	ctx := context.Background()
	unique := fmt.Sprintf("%d", time.Now().UnixNano())
	projectID := "prj_fixture_" + unique

	created, err := testHandler.Queries.CreateCloudRuntimeCredential(ctx, db.CreateCloudRuntimeCredentialParams{
		WorkspaceID:    parseUUID(testWorkspaceID),
		Name:           name,
		Provider:       "vercel_sandbox",
		EncryptedToken: "encrypted-token-" + unique,
		ProjectID:      projectID,
		TeamID:         pgtype.Text{String: "team-" + unique, Valid: true},
		BaseSnapshotID: pgtype.Text{String: "snap-" + unique, Valid: true},
		Region:         "iad1",
		Status:         "active",
		OwnerID:        parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("create credential fixture: %v", err)
	}

	id := uuidToString(created.ID)
	t.Cleanup(func() {
		_ = testHandler.Queries.DeleteCloudRuntimeCredential(context.Background(), db.DeleteCloudRuntimeCredentialParams{
			ID:          created.ID,
			WorkspaceID: parseUUID(testWorkspaceID),
		})
	})

	return credentialFixture{ID: id, ProjectID: projectID}
}
