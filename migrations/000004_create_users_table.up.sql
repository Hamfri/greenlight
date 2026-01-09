CREATE TABLE IF NOT EXISTS users(
    id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    name text NOT NULL,
    email CITEXT UNIQUE NOT NULL, -- case insensitive text
    password BYTEA NOT NULL, -- binary string
    activated BOOL NOT NULL,
    created_at TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 1
);
