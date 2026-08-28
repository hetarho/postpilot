-- +goose Up
-- Posts, their photos, and the in-flight uploads that become photos (PRD §5, plan 02).
CREATE TABLE posts (
    -- The slug is the primary key and appears in R2 object keys, so it is minted once
    -- and never changes — retitling a post does not rename it.
    slug         TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        TEXT NOT NULL DEFAULT '',
    memo         TEXT NOT NULL DEFAULT '',
    -- Written by the generation pipeline (plan 06); NULL until then, which is what
    -- distinguishes "never generated" from "generated empty".
    observations TEXT,
    content      TEXT,
    status       TEXT NOT NULL DEFAULT 'draft', -- draft | review
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

CREATE TABLE images (
    id        TEXT PRIMARY KEY,
    post_slug TEXT NOT NULL REFERENCES posts(slug) ON DELETE CASCADE,
    -- The name the model and the exporters refer to a photo by (PRD §5), which is why
    -- it has to be unique within a post.
    filename  TEXT NOT NULL,
    r2_key    TEXT NOT NULL,
    -- Measured in the browser: the server never decodes an image ([I6]).
    width     INTEGER NOT NULL,
    height    INTEGER NOT NULL,
    -- From the R2 HEAD at confirm time, not from the client.
    bytes     INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (post_slug, filename)
);

-- An upload that has been presigned but not yet confirmed. Not in PRD §5: it exists
-- because ConfirmUpload(upload_id) needs the server to remember what that id referred to
-- across a 10-minute window and a possible restart. Rows die on confirm or by the sweep.
CREATE TABLE uploads (
    id         TEXT PRIMARY KEY, -- becomes images.id on confirm
    post_slug  TEXT NOT NULL REFERENCES posts(slug) ON DELETE CASCADE,
    filename   TEXT NOT NULL,
    r2_key     TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    -- Mirrors the images constraint so two concurrent CreateUpload calls for one
    -- filename cannot both be presigned and then collide on the second confirm.
    UNIQUE (post_slug, filename)
);

-- The list screen's only query: this user's posts, newest first.
CREATE INDEX idx_posts_user_updated ON posts(user_id, updated_at);
CREATE INDEX idx_images_post_slug ON images(post_slug);
-- The orphan sweep scans by expiry.
CREATE INDEX idx_uploads_expires_at ON uploads(expires_at);
CREATE INDEX idx_uploads_post_slug ON uploads(post_slug);

-- +goose Down
DROP INDEX idx_uploads_post_slug;
DROP INDEX idx_uploads_expires_at;
DROP INDEX idx_images_post_slug;
DROP INDEX idx_posts_user_updated;
DROP TABLE uploads;
DROP TABLE images;
DROP TABLE posts;
