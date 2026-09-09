-- +goose Up
CREATE TABLE video_templates (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    information_fields TEXT NOT NULL,
    cut_guidance TEXT NOT NULL,
    copy_styles TEXT NOT NULL,
    accent TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(user_id, name),
    UNIQUE(id, user_id)
);

CREATE TABLE clip_projects (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    video_template_id TEXT,
    ratio TEXT NOT NULL CHECK(ratio IN ('vertical', 'horizontal', 'square')),
    target_duration_ms INTEGER NOT NULL CHECK(target_duration_ms BETWEEN 15000 AND 90000),
    analysis_json TEXT,
    edit_plan_json TEXT,
    result_key TEXT,
    result_content_type TEXT,
    result_bytes INTEGER,
    result_duration_ms INTEGER,
    result_created_at TEXT,
    edit_plan_revision INTEGER NOT NULL DEFAULT 0,
    rendered_plan_revision INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(id, user_id),
    FOREIGN KEY(video_template_id, user_id) REFERENCES video_templates(id, user_id)
);
CREATE INDEX clip_projects_owner_updated ON clip_projects(user_id, updated_at DESC, id);
CREATE INDEX clip_projects_template ON clip_projects(video_template_id, user_id);

CREATE TABLE clip_project_answers (
    project_id TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    answer TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(project_id, label),
    FOREIGN KEY(project_id, user_id) REFERENCES clip_projects(id, user_id) ON DELETE CASCADE
);

-- +goose StatementBegin
CREATE TRIGGER video_templates_detach BEFORE DELETE ON video_templates BEGIN
    UPDATE clip_projects SET video_template_id = NULL
    WHERE video_template_id = OLD.id AND user_id = OLD.user_id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER video_templates_detach;
DROP TABLE clip_project_answers;
DROP TABLE clip_projects;
DROP TABLE video_templates;
