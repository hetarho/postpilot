-- Clip owns the server-export allowance, separate from AI-credit lots.
-- name: InsertExportWindow :execrows
INSERT INTO server_export_windows(user_id,coverage_id,window_start,window_end,allowance,correlation_id)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id,coverage_id,window_start) DO UPDATE SET window_end=excluded.window_end
WHERE server_export_windows.window_end < excluded.window_end;

-- name: InsertExportAdjustment :execrows
INSERT INTO server_export_adjustments(correlation_id,user_id,coverage_id,window_start,allowance_delta)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(correlation_id) DO NOTHING;

-- name: RaiseExportAllowance :execrows
UPDATE server_export_windows SET allowance=allowance+?
WHERE user_id=? AND coverage_id=? AND window_start=?;

-- name: CurrentExportWindow :one
SELECT user_id,coverage_id,window_start,window_end,allowance,used,reserved,correlation_id
FROM server_export_windows
WHERE user_id=? AND window_start<=? AND window_end>?
ORDER BY window_start DESC LIMIT 1;
