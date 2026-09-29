-- +goose Up
-- Preserve existing plan values while replacing the users.plan CHECK. SQLite cannot
-- widen a column CHECK in place. DROP/ADD leaves the parent table and every child FK
-- intact, unlike a table rebuild.
CREATE TABLE migration_0114_user_plans (
    id TEXT PRIMARY KEY,
    plan TEXT NOT NULL
);
INSERT INTO migration_0114_user_plans(id, plan) SELECT id, plan FROM users;

ALTER TABLE users DROP COLUMN plan;
ALTER TABLE users ADD COLUMN plan TEXT NOT NULL DEFAULT 'free'
    CHECK (plan IN ('free','light','basic','pro','max','master'));
UPDATE users SET plan = (
    SELECT saved.plan FROM migration_0114_user_plans AS saved WHERE saved.id = users.id
);
DROP TABLE migration_0114_user_plans;

-- +goose Down
-- The old schema cannot store light. Map it to the nearest paid tier for rollback
-- without deleting accounts or their dependent data.
CREATE TABLE migration_0114_user_plans (
    id TEXT PRIMARY KEY,
    plan TEXT NOT NULL
);
INSERT INTO migration_0114_user_plans(id, plan)
SELECT id, CASE WHEN plan = 'light' THEN 'basic' ELSE plan END FROM users;

ALTER TABLE users DROP COLUMN plan;
ALTER TABLE users ADD COLUMN plan TEXT NOT NULL DEFAULT 'free'
    CHECK (plan IN ('free','basic','pro','max','master'));
UPDATE users SET plan = (
    SELECT saved.plan FROM migration_0114_user_plans AS saved WHERE saved.id = users.id
);
DROP TABLE migration_0114_user_plans;
