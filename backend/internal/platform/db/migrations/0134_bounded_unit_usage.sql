-- +goose Up
CREATE TABLE usage_unit_quotes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    digest TEXT NOT NULL,
    quote_json TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    consumed_job_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX usage_unit_quotes_expiry_idx ON usage_unit_quotes(expires_at) WHERE consumed_job_id = '';

CREATE TABLE usage_unit_admissions (
    job_id TEXT PRIMARY KEY,
    admission_id INTEGER NOT NULL UNIQUE REFERENCES usage_admissions(id) ON DELETE CASCADE,
    quote_id TEXT NOT NULL UNIQUE,
    budgets_json TEXT NOT NULL
);
CREATE TABLE usage_unit_calls (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL REFERENCES usage_unit_admissions(job_id) ON DELETE CASCADE,
    budget_fingerprint TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE INDEX usage_unit_calls_budget_idx ON usage_unit_calls(job_id, budget_fingerprint);

CREATE TABLE usage_unit_events (
    event_id INTEGER PRIMARY KEY REFERENCES usage_events(id) ON DELETE CASCADE,
    claim_id TEXT NOT NULL UNIQUE REFERENCES usage_unit_calls(id) ON DELETE CASCADE,
    provider_id TEXT NOT NULL,
    supplier_request_id TEXT NOT NULL,
    budget_fingerprint TEXT NOT NULL,
    evidence_json TEXT NOT NULL,
    reported_usd TEXT NOT NULL,
    exact_usd TEXT NOT NULL,
    cost_source TEXT NOT NULL CHECK (cost_source IN ('reported', 'estimated', 'unavailable')),
    chargeable INTEGER NOT NULL CHECK (chargeable IN (0, 1))
);
CREATE INDEX usage_unit_events_request_idx ON usage_unit_events(provider_id, supplier_request_id);

-- +goose Down
DROP TABLE usage_unit_events;
DROP TABLE usage_unit_calls;
DROP TABLE usage_unit_admissions;
DROP TABLE usage_unit_quotes;
