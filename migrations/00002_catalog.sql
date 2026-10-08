-- +goose Up
CREATE TABLE marketplaces (id uuid PRIMARY KEY, org_id uuid NOT NULL REFERENCES organizations(id), name text NOT NULL, shorten_policy text NOT NULL CHECK(shorten_policy IN ('shorten','direct')), created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, deleted_at timestamptz, purged_at timestamptz);
CREATE UNIQUE INDEX marketplaces_org_name_unique ON marketplaces(org_id, lower(name)) WHERE purged_at IS NULL;
CREATE TABLE channels (id uuid PRIMARY KEY, org_id uuid NOT NULL REFERENCES organizations(id), name text NOT NULL, segment text NOT NULL, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, deleted_at timestamptz, purged_at timestamptz);
CREATE UNIQUE INDEX channels_org_segment_unique ON channels(org_id, segment) WHERE purged_at IS NULL;
CREATE TABLE affiliate_links (id uuid PRIMARY KEY, org_id uuid NOT NULL REFERENCES organizations(id), marketplace_id uuid NOT NULL REFERENCES marketplaces(id), title text NOT NULL, image_url text, destination_url text NOT NULL, shorten_policy_override text CHECK(shorten_policy_override IN ('shorten','direct')), active boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL, deleted_at timestamptz, purged_at timestamptz);
CREATE TABLE short_codes (code char(7) PRIMARY KEY CHECK(code ~ '^[0-9a-z]{7}$'), org_id uuid NOT NULL REFERENCES organizations(id), target_type text NOT NULL CHECK(target_type='affiliate_link'), target_id uuid NOT NULL REFERENCES affiliate_links(id), created_at timestamptz NOT NULL);
CREATE UNIQUE INDEX short_codes_target_unique ON short_codes(target_type,target_id);
-- +goose Down
DROP TABLE short_codes; DROP TABLE affiliate_links; DROP TABLE channels; DROP TABLE marketplaces;
