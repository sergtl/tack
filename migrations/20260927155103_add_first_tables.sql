-- +goose Up
CREATE TABLE workspaces (
    id      INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name    VARCHAR (255) NOT NULL,
    slug    VARCHAR (255) NOT NULL UNIQUE
);

CREATE TABLE users (
    id              INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            VARCHAR (255) NOT NULL,
    email           VARCHAR (255) NOT NULL UNIQUE,
    password        VARCHAR (255) NOT NULL,
    workspace_id    int REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE TABLE projects (
    id              INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title           VARCHAR (255) NOT NULL,
    description     VARCHAR (255) NOT NULL,
    workspace_id    int REFERENCES workspaces(id) ON DELETE CASCADE
);

CREATE TABLE issues (
    id              INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title           VARCHAR (255) NOT NULL,
    description     VARCHAR (255) NOT NULL,
    status          VARCHAR (64) NOT NULL,
    assignee_id     int REFERENCES users(id) ON DELETE SET NULL, 
    project_id      int REFERENCES projects(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE issues;
DROP TABLE projects;
DROP TABLE users;
DROP TABLE workspaces;