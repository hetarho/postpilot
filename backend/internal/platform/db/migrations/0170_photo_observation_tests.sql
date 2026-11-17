-- +goose Up
-- MODEL r39 (T891): a writing test carries an output kind, and a photo observation test
-- (MODEL-97) owns the photos its owner uploads to it. Every test that exists before this
-- migration is a post test. The factor/stage pairing that an observation test needs is
-- enforced in Go: SQLite cannot add a table CHECK to an existing table here.
ALTER TABLE writing_tests ADD COLUMN output TEXT NOT NULL DEFAULT 'post' CHECK (output IN ('post', 'observation'));

-- A test photo is private test input, never a post attachment. It is created pending by an
-- upload handshake, becomes ready once the object is confirmed, and is attached to one test
-- when that test is admitted (test_id stays NULL until then, so the composite key is not
-- enforced for an unattached photo). Deleting the test row, or the account, deletes the photo
-- row; the stored object is removed by the stray sweep.
CREATE TABLE writing_test_photos (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    test_id     TEXT,
    filename    TEXT NOT NULL,
    object_key  TEXT NOT NULL UNIQUE,
    bytes       INTEGER NOT NULL DEFAULT 0 CHECK (bytes >= 0),
    width       INTEGER NOT NULL DEFAULT 0,
    height      INTEGER NOT NULL DEFAULT 0,
    status      TEXT NOT NULL CHECK (status IN ('pending', 'ready')),
    expires_at  TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    FOREIGN KEY (user_id, test_id) REFERENCES writing_tests(user_id, id) ON DELETE CASCADE
);
CREATE INDEX writing_test_photos_owner ON writing_test_photos(user_id, created_at);
CREATE INDEX writing_test_photos_expiry ON writing_test_photos(expires_at);

-- +goose Down
DROP TABLE writing_test_photos;
ALTER TABLE writing_tests DROP COLUMN output;
