ALTER TABLE movies ADD COLUMN title_tsvector TSVECTOR GENERATED ALWAYS AS (to_tsvector('simple',title)) STORED;
