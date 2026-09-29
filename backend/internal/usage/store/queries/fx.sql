-- Confirmed official FX publications and verified nonpublication dates.
-- name: GetRateDay :one
SELECT publication_date,state,reference_e4 FROM fx_reference_days
WHERE publication_date=?;

-- name: InsertRateDay :exec
INSERT INTO fx_reference_days(publication_date,state,source,reference_e4,verified_at)
VALUES (?,?,?,?,?) ON CONFLICT(publication_date) DO NOTHING;

-- name: LatestPublishedRateDay :one
SELECT publication_date,state,reference_e4 FROM fx_reference_days
WHERE state='published' AND publication_date<=?
ORDER BY publication_date DESC LIMIT 1;
