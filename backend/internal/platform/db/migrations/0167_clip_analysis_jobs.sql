-- +goose Up
-- CLIP-234: 관찰하기 runs as its own approved job of kind 'analyze_clip', so the trigger that
-- lets a clip quote be consumed only by the queued, unlinked job it approved admits that kind
-- beside the four it already named. The trigger is dropped and written back otherwise unchanged.
DROP TRIGGER clip_quote_job_owner;
-- +goose StatementBegin
CREATE TRIGGER clip_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_generation_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
      AND clip_project_id=NEW.project_id AND kind IN ('generate_clip','revise_clip','storyline_clip','revise_storyline_clip','analyze_clip') AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip quote job unavailable'); END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER clip_quote_job_owner;
-- +goose StatementBegin
CREATE TRIGGER clip_quote_job_owner BEFORE UPDATE OF consumed_job_id ON clip_generation_quotes
WHEN NEW.consumed_job_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM generation_jobs WHERE id=NEW.consumed_job_id AND user_id=NEW.user_id
      AND clip_project_id=NEW.project_id AND kind IN ('generate_clip','revise_clip','storyline_clip','revise_storyline_clip') AND status='queued' AND dispatch_ready=0
)
BEGIN SELECT RAISE(ABORT,'clip quote job unavailable'); END;
-- +goose StatementEnd
