-- +goose Up
-- CLIP-59, CLIP-112, CLIP-185: `<guide>` 는 영상 템플릿 문법에서 빠졌다. 템플릿은 형식만 담고,
-- 어떻게 보여줄지는 영상 지침이 맡는다. 저장된 템플릿 본문과 프로젝트가 고정해 둔 스냅샷 본문에서
-- 모든 `<guide>…</guide>` 와 `<guide/>` 를 지운다. 알파 전이라 읽기 호환 경로는 두지 않는다.
-- 지운 자리에 남는 빈 줄은 공백 글자라 그대로 읽히므로 본문을 다시 정리하지 않는다.
UPDATE video_templates SET composition_body = (
    WITH RECURSIVE strip(b) AS (
        SELECT replace(video_templates.composition_body, '<guide/>', '')
        UNION ALL
        SELECT substr(b, 1, instr(b, '<guide>') - 1) || substr(b, instr(b, '</guide>') + 8)
        FROM strip WHERE instr(b, '<guide>') > 0 AND instr(b, '</guide>') > instr(b, '<guide>')
    )
    SELECT b FROM strip WHERE NOT (instr(b, '<guide>') > 0 AND instr(b, '</guide>') > instr(b, '<guide>')) LIMIT 1
) WHERE instr(composition_body, '<guide') > 0;

UPDATE clip_projects SET composition_snapshot_json = json_set(composition_snapshot_json, '$.body', (
    WITH RECURSIVE strip(b) AS (
        SELECT replace(json_extract(clip_projects.composition_snapshot_json, '$.body'), '<guide/>', '')
        UNION ALL
        SELECT substr(b, 1, instr(b, '<guide>') - 1) || substr(b, instr(b, '</guide>') + 8)
        FROM strip WHERE instr(b, '<guide>') > 0 AND instr(b, '</guide>') > instr(b, '<guide>')
    )
    SELECT b FROM strip WHERE NOT (instr(b, '<guide>') > 0 AND instr(b, '</guide>') > instr(b, '<guide>')) LIMIT 1
)) WHERE json_valid(composition_snapshot_json) AND instr(json_extract(composition_snapshot_json, '$.body'), '<guide') > 0;

-- +goose Down
-- The removed guides are gone; there is nothing to restore.
SELECT 1;
