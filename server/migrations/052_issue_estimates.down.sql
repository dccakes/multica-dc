ALTER TABLE issue
    DROP CONSTRAINT IF EXISTS issue_estimate_source_check;

ALTER TABLE issue
    DROP COLUMN IF EXISTS estimate_source,
    DROP COLUMN IF EXISTS estimated_hours;
