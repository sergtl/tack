INSERT INTO workspaces (name, slug)
VALUES ('Development', 'development')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO users (name, email, password, workspace_id)
SELECT
    'Dev User',
    'dev@example.com',
    '$2a$14$OBaEp7RJGWaHdJQIrz2oquiQIF6qUdIDsAQqq1r3W8RvCm6mB9eMa', -- unhashed: dev-password --
    id
FROM workspaces
WHERE slug = 'development'
ON CONFLICT (email) DO NOTHING;