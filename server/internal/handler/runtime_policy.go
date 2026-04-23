package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/runtimepolicy"
)

type WorkspaceRuntimePolicyResponse struct {
	WorkspaceID                   string  `json:"workspace_id"`
	MonthlyBudgetCents            int64   `json:"monthly_budget_cents"`
	RemoteConcurrencyLimit        int32   `json:"remote_concurrency_limit"`
	DefaultParentIssueBudgetCents int64   `json:"default_parent_issue_budget_cents"`
	CreatedAt                     *string `json:"created_at,omitempty"`
	UpdatedAt                     *string `json:"updated_at,omitempty"`
}

type UpdateWorkspaceRuntimePolicyRequest struct {
	MonthlyBudgetCents            *int64 `json:"monthly_budget_cents"`
	RemoteConcurrencyLimit        *int32 `json:"remote_concurrency_limit"`
	DefaultParentIssueBudgetCents *int64 `json:"default_parent_issue_budget_cents"`
}

type IssueRuntimePolicyResponse struct {
	IssueID                string  `json:"issue_id"`
	BudgetCents            int64   `json:"budget_cents"`
	RemoteConcurrencyLimit int32   `json:"remote_concurrency_limit"`
	HasOverride            bool    `json:"has_override"`
	UpdatedBy              *string `json:"updated_by,omitempty"`
	CreatedAt              *string `json:"created_at,omitempty"`
	UpdatedAt              *string `json:"updated_at,omitempty"`
}

type UpdateIssueRuntimePolicyRequest struct {
	BudgetCents            *int64 `json:"budget_cents"`
	RemoteConcurrencyLimit *int32 `json:"remote_concurrency_limit"`
}

func (h *Handler) runtimePolicyStore() (*runtimepolicy.Store, bool) {
	if h.DB == nil {
		return nil, false
	}
	return runtimepolicy.NewStore(h.DB), true
}

func workspaceRuntimePolicyResponse(policy runtimepolicy.WorkspacePolicy) WorkspaceRuntimePolicyResponse {
	return WorkspaceRuntimePolicyResponse{
		WorkspaceID:                   policy.WorkspaceID,
		MonthlyBudgetCents:            policy.MonthlyBudgetCents,
		RemoteConcurrencyLimit:        policy.RemoteConcurrencyLimit,
		DefaultParentIssueBudgetCents: policy.DefaultParentIssueBudgetCents,
		CreatedAt:                     runtimePolicyTimestamp(policy.CreatedAt),
		UpdatedAt:                     runtimePolicyTimestamp(policy.UpdatedAt),
	}
}

func issueRuntimePolicyResponse(issueID string, override *runtimepolicy.IssueBudgetOverride, defaultBudget int64, defaultConcurrency int32) IssueRuntimePolicyResponse {
	resp := IssueRuntimePolicyResponse{
		IssueID:                issueID,
		BudgetCents:            defaultBudget,
		RemoteConcurrencyLimit: defaultConcurrency,
		HasOverride:            false,
	}
	if override == nil {
		return resp
	}
	resp.BudgetCents = override.BudgetCents
	if override.RemoteConcurrencyLimit.Valid {
		resp.RemoteConcurrencyLimit = override.RemoteConcurrencyLimit.Int32
	}
	resp.HasOverride = true
	resp.UpdatedBy = &override.UpdatedBy
	resp.CreatedAt = runtimePolicyTimestamp(override.CreatedAt)
	resp.UpdatedAt = runtimePolicyTimestamp(override.UpdatedAt)
	return resp
}

func runtimePolicyTimestamp(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

func (h *Handler) GetWorkspaceRuntimePolicy(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin"); !ok {
		return
	}
	store, ok := h.runtimePolicyStore()
	if !ok {
		writeError(w, http.StatusInternalServerError, "runtime policy store unavailable")
		return
	}
	policy, err := store.GetWorkspacePolicy(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace runtime policy")
		return
	}
	writeJSON(w, http.StatusOK, workspaceRuntimePolicyResponse(policy))
}

func (h *Handler) UpdateWorkspaceRuntimePolicy(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "id")
	if _, ok := h.requireWorkspaceRole(w, r, workspaceID, "workspace not found", "owner", "admin"); !ok {
		return
	}
	store, ok := h.runtimePolicyStore()
	if !ok {
		writeError(w, http.StatusInternalServerError, "runtime policy store unavailable")
		return
	}

	var req UpdateWorkspaceRuntimePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	current, err := store.GetWorkspacePolicy(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace runtime policy")
		return
	}
	if req.MonthlyBudgetCents != nil {
		current.MonthlyBudgetCents = *req.MonthlyBudgetCents
	}
	if req.RemoteConcurrencyLimit != nil {
		if *req.RemoteConcurrencyLimit < 1 {
			writeError(w, http.StatusBadRequest, "remote_concurrency_limit must be at least 1")
			return
		}
		current.RemoteConcurrencyLimit = *req.RemoteConcurrencyLimit
	}
	if req.DefaultParentIssueBudgetCents != nil {
		current.DefaultParentIssueBudgetCents = *req.DefaultParentIssueBudgetCents
	}

	saved, err := store.UpsertWorkspacePolicy(r.Context(), current)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save workspace runtime policy")
		return
	}
	writeJSON(w, http.StatusOK, workspaceRuntimePolicyResponse(saved))
}

func (h *Handler) GetIssueRuntimePolicy(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, issueID)
	if !ok {
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(issue.WorkspaceID), "issue not found", "owner", "admin"); !ok {
		return
	}

	store, ok := h.runtimePolicyStore()
	if !ok {
		writeError(w, http.StatusInternalServerError, "runtime policy store unavailable")
		return
	}

	workspacePolicy, err := store.GetWorkspacePolicy(r.Context(), uuidToString(issue.WorkspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace runtime policy")
		return
	}

	override, err := store.GetIssueBudgetOverride(r.Context(), issueID)
	if err != nil && err != pgx.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "failed to load issue runtime policy")
		return
	}
	var overridePtr *runtimepolicy.IssueBudgetOverride
	if err == nil {
		overridePtr = &override
	}

	writeJSON(w, http.StatusOK, issueRuntimePolicyResponse(issueID, overridePtr, workspacePolicy.DefaultParentIssueBudgetCents, 1))
}

func (h *Handler) UpdateIssueRuntimePolicy(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, issueID)
	if !ok {
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(issue.WorkspaceID), "issue not found", "owner", "admin"); !ok {
		return
	}
	store, ok := h.runtimePolicyStore()
	if !ok {
		writeError(w, http.StatusInternalServerError, "runtime policy store unavailable")
		return
	}

	var req UpdateIssueRuntimePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.BudgetCents == nil && req.RemoteConcurrencyLimit == nil {
		writeError(w, http.StatusBadRequest, "at least one field must be provided")
		return
	}

	workspacePolicy, err := store.GetWorkspacePolicy(r.Context(), uuidToString(issue.WorkspaceID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load workspace runtime policy")
		return
	}
	effective := issueRuntimePolicyResponse(issueID, nil, workspacePolicy.DefaultParentIssueBudgetCents, 1)
	if existing, err := store.GetIssueBudgetOverride(r.Context(), issueID); err == nil {
		effective = issueRuntimePolicyResponse(issueID, &existing, workspacePolicy.DefaultParentIssueBudgetCents, 1)
	}

	budgetCents := effective.BudgetCents
	if req.BudgetCents != nil {
		budgetCents = *req.BudgetCents
	}
	concurrency := effective.RemoteConcurrencyLimit
	if req.RemoteConcurrencyLimit != nil {
		if *req.RemoteConcurrencyLimit < 1 {
			writeError(w, http.StatusBadRequest, "remote_concurrency_limit must be at least 1")
			return
		}
		concurrency = *req.RemoteConcurrencyLimit
	}

	updated, err := store.UpsertIssueBudgetOverride(r.Context(), runtimepolicy.IssueBudgetOverride{
		IssueID:                issueID,
		BudgetCents:            budgetCents,
		RemoteConcurrencyLimit: pgtype.Int4{Int32: concurrency, Valid: true},
		UpdatedBy:              requestUserID(r),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save issue runtime policy")
		return
	}

	writeJSON(w, http.StatusOK, issueRuntimePolicyResponse(issueID, &updated, workspacePolicy.DefaultParentIssueBudgetCents, 1))
}

func (h *Handler) DeleteIssueRuntimePolicy(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "id")
	issue, ok := h.loadIssueForUser(w, r, issueID)
	if !ok {
		return
	}
	if _, ok := h.requireWorkspaceRole(w, r, uuidToString(issue.WorkspaceID), "issue not found", "owner", "admin"); !ok {
		return
	}
	store, ok := h.runtimePolicyStore()
	if !ok {
		writeError(w, http.StatusInternalServerError, "runtime policy store unavailable")
		return
	}
	if err := store.DeleteIssueBudgetOverride(r.Context(), issueID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear issue runtime policy")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) clearIssueRuntimePolicyOverride(ctx context.Context, issueID string) {
	store, ok := h.runtimePolicyStore()
	if !ok {
		return
	}
	_ = store.DeleteIssueBudgetOverride(ctx, issueID)
}
