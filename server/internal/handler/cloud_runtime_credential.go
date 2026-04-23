package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	vercelRuntimeCredentialProvider = "vercel_sandbox"
	defaultVercelRuntimeRegion      = "iad1"
)

type CloudRuntimeCredentialResponse struct {
	ID             string  `json:"id"`
	WorkspaceID    string  `json:"workspace_id"`
	Name           string  `json:"name"`
	Provider       string  `json:"provider"`
	ProjectID      string  `json:"project_id"`
	TeamID         *string `json:"team_id"`
	BaseSnapshotID *string `json:"base_snapshot_id"`
	Region         string  `json:"region"`
	Status         string  `json:"status"`
	LastTestedAt   *string `json:"last_tested_at"`
	LastTestError  *string `json:"last_test_error"`
	OwnerID        string  `json:"owner_id"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	HasToken       bool    `json:"has_token"`
}

type createVercelCloudRuntimeCredentialRequest struct {
	Name           string  `json:"name"`
	Token          string  `json:"token"`
	ProjectID      string  `json:"project_id"`
	TeamID         *string `json:"team_id"`
	BaseSnapshotID *string `json:"base_snapshot_id"`
	Region         string  `json:"region"`
}

type updateVercelCloudRuntimeCredentialRequest struct {
	Name           *string `json:"name"`
	ProjectID      *string `json:"project_id"`
	TeamID         *string `json:"team_id"`
	BaseSnapshotID *string `json:"base_snapshot_id"`
	Region         *string `json:"region"`
	Status         *string `json:"status"`
}

func cloudRuntimeCredentialToResponse(c db.CloudRuntimeCredential) CloudRuntimeCredentialResponse {
	return CloudRuntimeCredentialResponse{
		ID:             uuidToString(c.ID),
		WorkspaceID:    uuidToString(c.WorkspaceID),
		Name:           c.Name,
		Provider:       c.Provider,
		ProjectID:      c.ProjectID,
		TeamID:         textToPtr(c.TeamID),
		BaseSnapshotID: textToPtr(c.BaseSnapshotID),
		Region:         c.Region,
		Status:         c.Status,
		LastTestedAt:   timestampToPtr(c.LastTestedAt),
		LastTestError:  textToPtr(c.LastTestError),
		OwnerID:        uuidToString(c.OwnerID),
		CreatedAt:      timestampToString(c.CreatedAt),
		UpdatedAt:      timestampToString(c.UpdatedAt),
		HasToken:       c.EncryptedToken != "",
	}
}

func (h *Handler) ListVercelCloudRuntimeCredentials(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin"); !ok {
		return
	}

	credentials, err := h.Queries.ListCloudRuntimeCredentials(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list cloud runtime credentials")
		return
	}

	resp := make([]CloudRuntimeCredentialResponse, 0, len(credentials))
	for _, cred := range credentials {
		if cred.Provider != vercelRuntimeCredentialProvider {
			continue
		}
		resp = append(resp, cloudRuntimeCredentialToResponse(cred))
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) CreateVercelCloudRuntimeCredential(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	member, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin")
	if !ok {
		return
	}

	var req createVercelCloudRuntimeCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Token = strings.TrimSpace(req.Token)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Region = strings.TrimSpace(req.Region)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	if req.Region == "" {
		req.Region = defaultVercelRuntimeRegion
	}

	created, err := h.Queries.CreateCloudRuntimeCredential(r.Context(), db.CreateCloudRuntimeCredentialParams{
		WorkspaceID:    parseUUID(workspaceID),
		Name:           req.Name,
		Provider:       vercelRuntimeCredentialProvider,
		EncryptedToken: req.Token,
		ProjectID:      req.ProjectID,
		TeamID:         ptrToText(req.TeamID),
		BaseSnapshotID: ptrToText(req.BaseSnapshotID),
		Region:         req.Region,
		Status:         "active",
		OwnerID:        member.UserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create cloud runtime credential")
		return
	}

	writeJSON(w, http.StatusCreated, cloudRuntimeCredentialToResponse(created))
}

func (h *Handler) UpdateVercelCloudRuntimeCredential(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "credential not found", "owner", "admin"); !ok {
		return
	}

	cred, ok := h.loadVercelCloudRuntimeCredential(w, r, workspaceID)
	if !ok {
		return
	}

	var req updateVercelCloudRuntimeCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := cred.Name
	if req.Name != nil {
		v := strings.TrimSpace(*req.Name)
		if v == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		name = v
	}

	projectID := cred.ProjectID
	if req.ProjectID != nil {
		v := strings.TrimSpace(*req.ProjectID)
		if v == "" {
			writeError(w, http.StatusBadRequest, "project_id is required")
			return
		}
		projectID = v
	}

	teamID := cred.TeamID
	if req.TeamID != nil {
		teamID = ptrToText(req.TeamID)
	}

	baseSnapshotID := cred.BaseSnapshotID
	if req.BaseSnapshotID != nil {
		baseSnapshotID = ptrToText(req.BaseSnapshotID)
	}

	region := cred.Region
	if req.Region != nil {
		v := strings.TrimSpace(*req.Region)
		if v == "" {
			writeError(w, http.StatusBadRequest, "region is required")
			return
		}
		region = v
	}

	status := cred.Status
	if req.Status != nil {
		v := strings.TrimSpace(*req.Status)
		if v == "" {
			writeError(w, http.StatusBadRequest, "status is required")
			return
		}
		status = v
	}

	updated, err := h.Queries.UpdateCloudRuntimeCredential(r.Context(), db.UpdateCloudRuntimeCredentialParams{
		Name:           name,
		ProjectID:      projectID,
		TeamID:         teamID,
		BaseSnapshotID: baseSnapshotID,
		Region:         region,
		Status:         status,
		LastTestedAt:   cred.LastTestedAt,
		LastTestError:  cred.LastTestError,
		ID:             cred.ID,
		WorkspaceID:    parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update cloud runtime credential")
		return
	}

	writeJSON(w, http.StatusOK, cloudRuntimeCredentialToResponse(updated))
}

func (h *Handler) DeleteVercelCloudRuntimeCredential(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "credential not found", "owner", "admin"); !ok {
		return
	}

	cred, ok := h.loadVercelCloudRuntimeCredential(w, r, workspaceID)
	if !ok {
		return
	}

	if err := h.Queries.DeleteCloudRuntimeCredential(r.Context(), db.DeleteCloudRuntimeCredentialParams{
		ID:          cred.ID,
		WorkspaceID: parseUUID(workspaceID),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete cloud runtime credential")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TestVercelCloudRuntimeCredential(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "credential not found", "owner", "admin"); !ok {
		return
	}

	cred, ok := h.loadVercelCloudRuntimeCredential(w, r, workspaceID)
	if !ok {
		return
	}

	testedAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	updated, err := h.Queries.UpdateCloudRuntimeCredential(r.Context(), db.UpdateCloudRuntimeCredentialParams{
		Name:           cred.Name,
		ProjectID:      cred.ProjectID,
		TeamID:         cred.TeamID,
		BaseSnapshotID: cred.BaseSnapshotID,
		Region:         cred.Region,
		Status:         cred.Status,
		LastTestedAt:   testedAt,
		LastTestError:  pgtype.Text{},
		ID:             cred.ID,
		WorkspaceID:    parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to test cloud runtime credential")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"credential_id": uuidToString(updated.ID),
		"provider":      "vercel",
		"ok":            true,
		"status":        "passed",
		"checked_at":    timestampToString(updated.LastTestedAt),
		"message":       "stubbed vercel credential test passed",
	})
}

func (h *Handler) BootstrapVercelCloudRuntimeCredential(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "credential not found", "owner", "admin"); !ok {
		return
	}

	cred, ok := h.loadVercelCloudRuntimeCredential(w, r, workspaceID)
	if !ok {
		return
	}

	metadata := map[string]any{
		"runner_type":       "vercel_sandbox",
		"credential_id":     uuidToString(cred.ID),
		"project_id":        cred.ProjectID,
		"region":            cred.Region,
		"base_snapshot_id":  cred.BaseSnapshotID.String,
		"credential_status": cred.Status,
	}
	metadataBytes, _ := json.Marshal(metadata)
	runtime, err := h.Queries.UpsertAgentRuntime(r.Context(), db.UpsertAgentRuntimeParams{
		WorkspaceID: parseUUID(workspaceID),
		DaemonID:    strToText("cloudrunner:" + uuidToString(cred.ID)),
		Name:        fmt.Sprintf("Vercel Cloud (%s)", cred.ProjectID),
		RuntimeMode: "cloud",
		Provider:    "codex",
		Status:      "offline",
		DeviceInfo:  "Vercel Sandbox",
		Metadata:    metadataBytes,
		OwnerID:     cred.OwnerID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to bootstrap cloud runtime")
		return
	}
	if h.DB != nil {
		if _, err := h.DB.Exec(r.Context(), `
			UPDATE agent_runtime
			SET credential_id = $2,
			    updated_at = now()
			WHERE id = $1
		`, runtime.ID, cred.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to link runtime credential")
			return
		}
	}

	resp := map[string]any{
		"credential_id": uuidToString(cred.ID),
		"provider":      "vercel",
		"status":        "ready",
		"project_id":    cred.ProjectID,
		"region":        cred.Region,
		"command":       fmt.Sprintf("vercel link --project %s", cred.ProjectID),
		"runtime_id":    uuidToString(runtime.ID),
	}
	if cred.TeamID.Valid {
		resp["team_id"] = cred.TeamID.String
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) loadVercelCloudRuntimeCredential(w http.ResponseWriter, r *http.Request, workspaceID string) (db.CloudRuntimeCredential, bool) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "credential id is required")
		return db.CloudRuntimeCredential{}, false
	}

	cred, err := h.Queries.GetCloudRuntimeCredential(r.Context(), db.GetCloudRuntimeCredentialParams{
		ID:          parseUUID(id),
		WorkspaceID: parseUUID(workspaceID),
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "credential not found")
		return db.CloudRuntimeCredential{}, false
	}

	if cred.Provider != vercelRuntimeCredentialProvider {
		writeError(w, http.StatusNotFound, "credential not found")
		return db.CloudRuntimeCredential{}, false
	}

	return cred, true
}
