-- +goose Up
-- MEM-15, MEM-16: a finished memory extraction's payload — its frozen post body and raw
-- candidate list — is cleared once its candidates are ruled on. Before this there was no ruling
-- on the server, so a finished extraction kept both for good; pre-alpha, every finished one is
-- treated as ruled on and emptied. The extraction is repeatable on demand. A queued or running
-- one keeps its frozen source, which it still needs.
UPDATE generation_jobs SET payload = ''
WHERE kind = 'extract_memory' AND status = 'done' AND payload <> '';

-- +goose Down
-- The cleared text is gone; there is nothing to restore.
SELECT 1;
