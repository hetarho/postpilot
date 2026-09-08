-- +goose Up
-- The answers a post gives to the data fields its template declared (POST-62, TEMPLATE-43).
--
-- One row per label rather than one blob on posts: a single field edited by autosave is one
-- upsert, not a read-modify-write of the whole set, so two tabs answering two fields cannot
-- overwrite each other. It inherits the images/videos shape — owned by the post, gone with it.
--
-- The key is the LABEL, which is text the template authored and not an id. Nothing here points
-- at a template: assigning, renaming, swapping or deleting one must not destroy what a person
-- typed, and an answer whose label the current template no longer carries is simply never
-- shown. Rows are only ever inserted or updated; clearing an answer is an empty `answer`,
-- which the freeze already treats exactly like `enabled = 0`.
CREATE TABLE post_template_answers (
    post_slug  TEXT    NOT NULL REFERENCES posts(slug) ON DELETE CASCADE,
    label      TEXT    NOT NULL,
    answer     TEXT    NOT NULL DEFAULT '',
    enabled    INTEGER NOT NULL DEFAULT 1,
    updated_at TEXT    NOT NULL,
    PRIMARY KEY (post_slug, label)
);

-- +goose Down
DROP TABLE post_template_answers;
