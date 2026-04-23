ALTER TABLE issue
    ADD COLUMN IF NOT EXISTS estimated_hours DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS estimate_source TEXT;

ALTER TABLE issue
    ADD CONSTRAINT issue_estimate_source_check
    CHECK (estimate_source IS NULL OR estimate_source IN ('human', 'agent'));
