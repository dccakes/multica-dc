DROP INDEX IF EXISTS idx_agent_runtime_credential_id;

ALTER TABLE agent_runtime
    DROP COLUMN IF EXISTS credential_id;

DROP INDEX IF EXISTS idx_cloud_runtime_session_runtime;
DROP INDEX IF EXISTS uq_cloud_runtime_session_chat;
DROP INDEX IF EXISTS uq_cloud_runtime_session_issue;
DROP TABLE IF EXISTS cloud_runtime_session;

DROP INDEX IF EXISTS idx_cloud_runtime_credential_owner;
DROP INDEX IF EXISTS idx_cloud_runtime_credential_workspace;
DROP TABLE IF EXISTS cloud_runtime_credential;
