CREATE TABLE organizations (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (organization_id, user_id)
);

CREATE INDEX memberships_user_id_idx ON memberships(user_id);

CREATE TABLE invitations (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	email TEXT NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
	token_hash BYTEA NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	accepted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX invitations_organization_id_idx ON invitations(organization_id);

CREATE TABLE api_keys (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	prefix TEXT NOT NULL,
	key_hash BYTEA NOT NULL,
	scopes TEXT[] NOT NULL DEFAULT '{}',
	expires_at TIMESTAMPTZ,
	last_used_at TIMESTAMPTZ,
	revoked_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX api_keys_key_hash_idx ON api_keys(key_hash);
CREATE INDEX api_keys_organization_id_idx ON api_keys(organization_id);

CREATE TABLE widgets (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	version BIGINT NOT NULL DEFAULT 1,
	deleted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE operations (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	state TEXT NOT NULL,
	result JSONB,
	error JSONB,
	lease_owner TEXT,
	lease_expires_at TIMESTAMPTZ,
	retry_count INTEGER NOT NULL DEFAULT 0,
	next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	event_type TEXT NOT NULL,
	payload JSONB NOT NULL,
	state TEXT NOT NULL DEFAULT 'pending',
	lease_owner TEXT,
	lease_expires_at TIMESTAMPTZ,
	retry_count INTEGER NOT NULL DEFAULT 0,
	next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_events (
	id TEXT PRIMARY KEY,
	organization_id TEXT,
	actor_type TEXT NOT NULL,
	actor_id TEXT,
	action TEXT NOT NULL,
	resource_type TEXT NOT NULL,
	resource_id TEXT,
	result TEXT NOT NULL,
	request_id TEXT,
	metadata JSONB NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE objects (
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	key TEXT NOT NULL,
	content_type TEXT NOT NULL,
	size BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (organization_id, key)
);

CREATE TABLE webhook_endpoints (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	url TEXT NOT NULL,
	event_types TEXT[] NOT NULL,
	secret_hash BYTEA NOT NULL,
	secret_ciphertext BYTEA NOT NULL,
	secret_nonce BYTEA NOT NULL,
	disabled_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webhook_deliveries (
	id TEXT PRIMARY KEY,
	organization_id TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	endpoint_id TEXT NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
	event_id TEXT NOT NULL,
	event_type TEXT NOT NULL,
	payload JSONB NOT NULL,
	state TEXT NOT NULL DEFAULT 'pending',
	attempts INTEGER NOT NULL DEFAULT 0,
	next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_status_code INTEGER,
	last_error TEXT,
	delivered_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

