-- +goose Up
-- TMPL-18, TMPL-57: `<note>` 는 글 템플릿 문법에서 빠졌다. 템플릿은 형식만 담고, 어떻게 쓸지는 지침이
-- 맡는다. 빌더와 형식 안내는 `<note>…</note>` 쌍만 썼으므로 저장된 본문에서 그 쌍을 모두 지운다.
-- 지운 뒤에도 `<note` 가 남는 본문은 그대로 둔다. 알파 전이라 읽기 호환 경로는 두지 않으며, 제목
-- 영역은 note 를 받은 적이 없어 본문만 고친다. 지운 자리에 남는 빈 줄은 공백 글자라 그대로 읽힌다.
UPDATE templates SET body = (
    WITH RECURSIVE strip(b) AS (
        SELECT templates.body
        UNION ALL
        SELECT substr(b, 1, instr(b, '<note>') - 1) || substr(b, instr(b, '</note>') + 7)
        FROM strip WHERE instr(b, '<note>') > 0 AND instr(b, '</note>') > instr(b, '<note>')
    )
    SELECT b FROM strip WHERE NOT (instr(b, '<note>') > 0 AND instr(b, '</note>') > instr(b, '<note>')) LIMIT 1
) WHERE instr(body, '<note>') > 0;

-- +goose Down
-- The removed notes are gone; there is nothing to restore.
SELECT 1;
