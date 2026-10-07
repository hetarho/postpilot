-- +goose NO TRANSACTION
-- +goose Up
-- Sparse writing-test preparation needs exactly the remaining1..16 candidates.
-- This parent rebuild follows0129: disable foreign keys outside the explicit
-- transaction so dropping the old parent preserves operations and mutation
-- receipts. Check this owned graph before committing and restore enforcement.
PRAGMA foreign_keys=OFF;
BEGIN;
CREATE TABLE configuration_authoring_sessions_exact (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('post_template','video_template','post_guideline','video_guideline','writing_voice')),
 target_id TEXT NOT NULL DEFAULT '',
 request_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision >= 0),
 phase TEXT NOT NULL,
 snapshot TEXT NOT NULL,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 saved_baseline TEXT CHECK(saved_baseline IS NULL OR json_valid(saved_baseline)),
 working_source TEXT CHECK(working_source IS NULL OR json_valid(working_source)),
 draft_state TEXT NOT NULL DEFAULT 'valid' CHECK(draft_state IN ('valid','incomplete','invalid')),
 has_unpublished_changes INTEGER NOT NULL DEFAULT 0 CHECK(has_unpublished_changes IN (0,1)),
 saved_available INTEGER NOT NULL DEFAULT 0 CHECK(saved_available IN (0,1)),
 publication_pending INTEGER NOT NULL DEFAULT 0 CHECK(publication_pending IN (0,1)),
 target_conflict INTEGER NOT NULL DEFAULT 0 CHECK(target_conflict IN (0,1)),
 display_name TEXT NOT NULL DEFAULT '',
 candidate_count INTEGER NOT NULL DEFAULT 8 CHECK(candidate_count IN (1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16)),
 UNIQUE(user_id,request_id),
 UNIQUE(user_id,id)
);
INSERT INTO configuration_authoring_sessions_exact (
 id,user_id,kind,target_id,request_id,revision,phase,snapshot,created_at,updated_at,
 saved_baseline,working_source,draft_state,has_unpublished_changes,saved_available,
 publication_pending,target_conflict,display_name,candidate_count
)
SELECT id,user_id,kind,target_id,request_id,revision,phase,snapshot,created_at,updated_at,
 saved_baseline,working_source,draft_state,has_unpublished_changes,saved_available,
 publication_pending,target_conflict,display_name,candidate_count
FROM configuration_authoring_sessions;
DROP TABLE configuration_authoring_sessions;
ALTER TABLE configuration_authoring_sessions_exact RENAME TO configuration_authoring_sessions;
CREATE INDEX configuration_authoring_latest ON configuration_authoring_sessions(user_id,kind,target_id,updated_at DESC,id DESC);
CREATE INDEX configuration_authoring_summaries ON configuration_authoring_sessions(user_id,kind,saved_available,updated_at DESC,id DESC);
CREATE TABLE migration_0152_integrity_guard (problem TEXT NOT NULL CHECK(problem = ''));
INSERT INTO migration_0152_integrity_guard(problem)
SELECT 'migration left an authoring foreign-key violation'
WHERE EXISTS(SELECT 1 FROM pragma_foreign_key_check('configuration_authoring_sessions'))
 OR EXISTS(SELECT 1 FROM pragma_foreign_key_check('configuration_authoring_operations'))
 OR EXISTS(SELECT 1 FROM pragma_foreign_key_check('configuration_authoring_mutations'));
DROP TABLE migration_0152_integrity_guard;
COMMIT;
PRAGMA foreign_keys=ON;

-- +goose Down
-- Forward-only: binary rollback must preserve every paid draft and receipt.
SELECT 1;
