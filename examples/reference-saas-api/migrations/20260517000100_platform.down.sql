-- Local/schema-teardown helper only. Do not run in production.
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
DROP TABLE IF EXISTS objects;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS operations;
DROP TABLE IF EXISTS widgets;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS organizations;
