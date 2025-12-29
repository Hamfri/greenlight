ALTER TABLE movies ADD CONSTRAINT movies_runtime_check CHECK (runtime >= 0);

ALTER TABLE movies ADD CONSTRAINT movies_year_check CHECK (year >= 1888 AND year::double precision <= date_part('year'::text, now()));

ALTER TABLE movies ADD CONSTRAINT movies_genres_length_check CHECK (array_length(genres, 1) <= 5);
