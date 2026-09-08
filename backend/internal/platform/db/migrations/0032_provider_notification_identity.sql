-- +goose Up
-- One provider state, one row. An ordinary Toss payment webhook carries no event id of its
-- own, so the state transition itself — (provider, order_id, status, event_type) — is the
-- only identity available, and a redelivery of the same transition is the same fact. The
-- first arrival's received_at is what matters, so the repeat is dropped rather than appended.
--
-- The DELETE clears whatever the table already holds under that rule, keeping the earliest
-- row of each group. It is confined to rows whose order and status are both known: SQLite
-- groups NULLs together even though the unique index treats them as distinct, and rows with
-- an unknown order have no identity to deduplicate on.
DELETE FROM provider_notifications
WHERE order_id IS NOT NULL AND status IS NOT NULL
  AND id NOT IN (
      SELECT min(id) FROM provider_notifications
      WHERE order_id IS NOT NULL AND status IS NOT NULL
      GROUP BY provider, order_id, status, event_type
  );

CREATE UNIQUE INDEX idx_provider_notifications_state
    ON provider_notifications(provider, order_id, status, event_type);

-- +goose Down
DROP INDEX idx_provider_notifications_state;
