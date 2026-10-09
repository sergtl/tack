-- +goose Up
ALTER TABLE projects
ADD CONSTRAINT projects_id_workspace_uq
UNIQUE (id, workspace_id);

ALTER TABLE issues
ADD COLUMN workspace_id INTEGER NOT NULL
REFERENCES workspaces(id) ON DELETE CASCADE;

ALTER TABLE issues
DROP CONSTRAINT issues_project_id_fkey;

ALTER TABLE issues
ADD CONSTRAINT issues_project_workspace_fk
FOREIGN KEY (project_id, workspace_id)
REFERENCES projects (id, workspace_id)
ON DELETE SET NULL (project_id);

ALTER TABLE projects
ALTER COLUMN workspace_id SET NOT NULL;

-- +goose Down
ALTER TABLE issues
DROP CONSTRAINT issues_project_workspace_fk;

ALTER TABLE issues
DROP COLUMN workspace_id;

ALTER TABLE projects
DROP CONSTRAINT projects_id_workspace_uq;

ALTER TABLE issues
ADD CONSTRAINT issues_project_id_fkey
FOREIGN KEY (project_id)
REFERENCES projects(id)
ON DELETE CASCADE;

ALTER TABLE projects
ALTER COLUMN workspace_id DROP NOT NULL;