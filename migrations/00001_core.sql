-- +goose Up
CREATE TABLE organizations (id uuid PRIMARY KEY, name text NOT NULL, slug text UNIQUE NOT NULL, created_at timestamptz NOT NULL);
CREATE TABLE users (id uuid PRIMARY KEY, email text UNIQUE NOT NULL CHECK (email = lower(email)), password_hash text NOT NULL, created_at timestamptz NOT NULL, deleted_at timestamptz);
CREATE TABLE memberships (user_id uuid NOT NULL REFERENCES users(id), org_id uuid NOT NULL REFERENCES organizations(id), role text NOT NULL CHECK (role IN ('admin','maintainer')), PRIMARY KEY (user_id, org_id));
CREATE TABLE sessions (token_hash bytea PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id), org_id uuid NOT NULL REFERENCES organizations(id), csrf_token text NOT NULL, created_at timestamptz NOT NULL, last_seen_at timestamptz NOT NULL, expires_at timestamptz NOT NULL);
CREATE TABLE app_settings (key text PRIMARY KEY, value text NOT NULL);
-- +goose Down
DROP TABLE app_settings; DROP TABLE sessions; DROP TABLE memberships; DROP TABLE users; DROP TABLE organizations;
