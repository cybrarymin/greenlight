CREATE TABLE movies(
    id BIGSERIAL PRIMARY KEY NOT NULL,
    created_at  TIMESTAMP(0) WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    title TEXT NOT NULL,
    year INTEGER NOT NULL,
    runtime INTEGER NOT NULL,
    genres TEXT[] NOT NULL,
    version INTEGER NOT NULL DEFAULT 1
);