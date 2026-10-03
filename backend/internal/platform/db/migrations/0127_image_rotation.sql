-- +goose Up
-- A photo carries a clockwise turn (POST-107): `rotation` in degrees (0, 90, 180, 270) and
-- whether the owner set it. An observation sets the turn only while `rotation_by_owner` is 0
-- (GEN-79). Every existing photo reads as unturned and never turned by its owner.
ALTER TABLE images ADD COLUMN rotation INTEGER NOT NULL DEFAULT 0 CHECK (rotation IN (0, 90, 180, 270));
ALTER TABLE images ADD COLUMN rotation_by_owner INTEGER NOT NULL DEFAULT 0 CHECK (rotation_by_owner IN (0, 1));
