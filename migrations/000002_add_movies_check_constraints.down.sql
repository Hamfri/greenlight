ALTER TABLE movies
    DROP CONSTRAINT IF EXISTS movies_runtime_check,
    DROP CONSTRAINT IF EXISTS movies_year_check,
    DROP CONSTRAINT IF EXISTS movies_genres_length_check;
