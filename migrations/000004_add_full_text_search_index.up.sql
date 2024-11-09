CREATE INDEX IF NOT EXISTS title_tsvector_idx ON movies USING GIN (title_tsvector);
CREATE INDEX IF NOT EXISTS movies_genres_idx ON movies USING GIN (genres);