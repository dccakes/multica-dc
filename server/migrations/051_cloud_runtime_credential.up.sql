CREATE TABLE cloud_runtime_credential (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    encrypted_token TEXT NOT NULL,
    project_id TEXT NOT NULL,
    team_id TEXT,
    base_snapshot_id TEXT,
    region TEXT NOT NULL DEFAULT 'iad1',
    status TEXT NOT NULL DEFAULT 'active',
    last_tested_at TIMESTAMPTZ,
    last_test_error TEXT,
    owner_id UUID NOT NULL REFERENCES "user"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cloud_runtime_credential_workspace
    ON cloud_runtime_credential(workspace_id, created_at DESC);

CREATE INDEX idx_cloud_runtime_credential_owner
    ON cloud_runtime_credential(owner_id, created_at DESC);

CREATE TABLE cloud_runtime_session (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    runtime_id UUID NOT NULL REFERENCES agent_runtime(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agent(id) ON DELETE CASCADE,
    issue_id UUID REFERENCES issue(id) ON DELETE CASCADE,
    chat_session_id UUID REFERENCES chat_session(id) ON DELETE CASCADE,
    last_sandbox_id TEXT,
    last_snapshot_id TEXT,
    snapshot_created_at TIMESTAMPTZ,
    snapshot_expires_at TIMESTAMPTZ,
    last_workdir TEXT,
    last_branch TEXT,
    last_codex_session_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT cloud_runtime_session_context_ck
        CHECK ((issue_id IS NOT NULL) <> (chat_session_id IS NOT NULL))
);

CREATE UNIQUE INDEX uq_cloud_runtime_session_issue
    ON cloud_runtime_session(runtime_id, agent_id, issue_id)
    WHERE issue_id IS NOT NULL;

CREATE UNIQUE INDEX uq_cloud_runtime_session_chat
    ON cloud_runtime_session(runtime_id, agent_id, chat_session_id)
    WHERE chat_session_id IS NOT NULL;

CREATE INDEX idx_cloud_runtime_session_runtime
    ON cloud_runtime_session(runtime_id, updated_at DESC);

ALTER TABLE agent_runtime
    ADD COLUMN credential_id UUID REFERENCES cloud_runtime_credential(id) ON DELETE SET NULL;

CREATE INDEX idx_agent_runtime_credential_id
    ON agent_runtime(credential_id)
    WHERE credential_id IS NOT NULL;
