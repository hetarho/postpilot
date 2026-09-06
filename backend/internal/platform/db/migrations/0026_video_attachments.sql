-- +goose Up
-- Videos as the post's second attachment kind (VIDEO r1).
--
-- A video is its own table rather than a discriminated `images` row: it carries a duration
-- and a container content type a photo never has, and every query that means "the photos"
-- would otherwise have to remember to filter. The filename namespace is still ONE per post —
-- the model and the exporters address every attachment by filename alone — but that
-- uniqueness spans two tables and is therefore enforced in the post service's create and
-- confirm paths, not by a constraint (VIDEO-5).
CREATE TABLE videos (
    id        TEXT PRIMARY KEY,
    post_slug TEXT NOT NULL REFERENCES posts(slug) ON DELETE CASCADE,
    filename  TEXT NOT NULL,
    -- posts/{slug}/{id}.{ext} in the same private bucket as the photos, so the orphan
    -- sweep's single prefix listing already covers video objects (VIDEO-6).
    r2_key    TEXT NOT NULL,
    -- The container's type, part of the PUT signature and what the player needs.
    content_type TEXT NOT NULL,
    -- From the storage HEAD at confirm time, not from the client.
    bytes     INTEGER NOT NULL,
    -- Client-reported, like a photo's dimensions: the server never opens a container
    -- ([I6] extended to video, VIDEO-4). Bounded at confirm by VIDEO_MAX_SECONDS.
    duration_ms INTEGER NOT NULL,
    width     INTEGER NOT NULL,
    height    INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE (post_slug, filename)
);

CREATE INDEX idx_videos_post_slug ON videos(post_slug);

-- Pending uploads of both kinds share this table: an upload row exists to remember what an
-- upload_id referred to, and that job is identical for a photo and a clip. The defaults make
-- every existing row read as exactly what it is — a JPEG photo upload.
ALTER TABLE uploads ADD COLUMN kind TEXT NOT NULL DEFAULT 'photo';
ALTER TABLE uploads ADD COLUMN content_type TEXT NOT NULL DEFAULT 'image/jpeg';

-- Whether the model takes video INPUT. Like every other capability column, 0 means the
-- catalog has not been refreshed since videos existed, which reads as "cannot" until an
-- operator refreshes — the safe direction for a gate that refuses a run (VIDEO-11).
-- The mapping from the source's input_modalities is a separate change.
ALTER TABLE catalog_models ADD COLUMN video_input INTEGER NOT NULL DEFAULT 0 CHECK (video_input IN (0,1));

-- +goose Down
ALTER TABLE catalog_models DROP COLUMN video_input;
ALTER TABLE uploads DROP COLUMN content_type;
ALTER TABLE uploads DROP COLUMN kind;
DROP INDEX idx_videos_post_slug;
DROP TABLE videos;
