CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        created_at TIMESTAMP(0) WITH time zone NOT NULL DEFAULT NOW(),
        name TEXT NOT NULL,
        email CITEXT UNIQUE NOT NULL,
        password_hash BYTEA NOT NULL,
        activated BOOL NOT NULL,
        version INTEGER NOT NULL DEFAULT 1
);