-- +goose Up
-- A verdict may attach badges to each candidate, chosen or not (MODEL-61). They are verdict
-- metadata: written once with the verdict, never changed afterwards, and never weighed into
-- a rating (MODEL-63).
--
-- The primary key is what makes a repeat harmless: the same badge offered twice for the same
-- candidate is one row. `note` belongs to `other` alone, and the CHECK says so rather than
-- leaving a note stranded on a badge that does not explain it.
CREATE TABLE model_experiment_badges (
    experiment_id TEXT NOT NULL REFERENCES model_experiments(id) ON DELETE CASCADE,
    candidate_id  TEXT NOT NULL,
    badge         TEXT NOT NULL CHECK (badge IN (
                      'fast','natural','on_brief','structured','accurate','in_voice','concise',
                      'slow','ai_like','off_brief','verbose','inaccurate','off_voice',
                      'repetitive','broken_format','other')),
    note          TEXT CHECK (note IS NULL OR badge = 'other'),
    PRIMARY KEY (experiment_id, candidate_id, badge)
);

-- The leaderboard tallies badges by the model that earned them, which means joining back to
-- the candidate rows; the board reads one stage and window at a time.
CREATE INDEX model_experiment_badges_candidate
ON model_experiment_badges(candidate_id);

-- +goose Down
DROP INDEX model_experiment_badges_candidate;
DROP TABLE model_experiment_badges;
