ALTER TABLE movies ADD CONSTRAINT movies_runtime_check CHECK (runtime >= 0);
ALTER TABLE movies ADD CONSTRAINT movies_year_check CHECK (year BETWEEN 1888 AND EXTRACT(YEAR FROM NOW()));
ALTER TABLE movies ADD CONSTRAINT movies_genres_lenght_check CHECK (ARRAY_LENGTH(genres,1) BETWEEN 1 AND 5);
