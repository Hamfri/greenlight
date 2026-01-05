package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"greenlight/internal/validator"
	"time"

	"github.com/lib/pq"
)

type Movie struct {
	// since we are using BIGINT on postgreSQL it's better to use int64 here to avoid errors
	// Go infers `int` type from os which would be int64 on 64 bit systems and int 32 on 32 bit systems
	ID    int64  `json:"id"`
	Title string `json:"title"`
	// here we cap int to int32 so that it matches with postgres Integer type and avoid introducing errors when we exceed int32's upper and lower limits
	// or integer overflow errors see >>> https://go.dev/ref/spec#Integer_overflow
	Year      int32     `json:"year,omitzero"`    // omitzero introduced in go 1.24 removes the field if it has the zero value of the type
	Runtime   Runtime   `json:"runtime,omitzero"` // Movie runtime in minutes
	Genres    []string  `json:"genres,omitempty"` // Slice of genres for the movie (romance, comedy, etc.) omitempty useful for slices & maps
	Version   int32     `json:"version"`          // Version starts at one and will be incremented each time movie information is updated
	CreatedAt time.Time `json:"-"`                // -  omits the field entirely from json response
}

func ValidateMovie(v *validator.Validator, m *Movie) map[string]string {
	v.Check(m.Title != "", "title", "must be provided")
	v.Check(len(m.Title) <= 500, "title", "must not be more than 500 bytes long")

	v.Check(m.Year != 0, "year", "must be provided")
	v.Check(m.Year >= 1888, "year", "must be greater than 1888")
	v.Check(m.Year <= int32(time.Now().Year()), "year", "must not be in the future")

	v.Check(m.Runtime != 0, "runtime", "must be provided")
	v.Check(m.Runtime > 0, "runtime", "must be a postive integer")

	v.Check(m.Genres != nil, "genres", "must be provided")
	v.Check(len(m.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(m.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(m.Genres), "genres", "must not contain duplicate values")

	if !v.Valid() {
		return v.Errors
	}
	return nil
}

// wrap sql.DB connection pool
type MovieModel struct {
	DB *sql.DB
}

func (m MovieModel) Insert(movie *Movie) error {
	query := `
		INSERT INTO movies (title, year, runtime, genres)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, version
	`
	// slice containing the values for the placeholder parameters
	// it's good practice to put args in a slice if we are passing more than 3 args
	// pq.Array converts []string to pq.StringArray
	args := []any{movie.Title, movie.Year, movie.Runtime, pq.Array(movie.Genres)}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// .Scan copies values of ID, createdAt and Version from the DB
	// .Scan can only write to a pointer type
	return m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}

// fulltext search does not support searching parts of a word eg bookshelf -> book
// to search parts of a word consider using `pg_trgm` or `ILIKE`
// `ILIKE` performs full table scans therefore not ideal
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, error) {
	query := fmt.Sprintf(`
		SELECT id, title, year, runtime, genres, version, created_at
		FROM movies
		-- deprecated WHERE (LOWER(title) = LOWER($1) OR $1 = '')
		-- to_tsvector:- splits title into lexemes and removes commonly occuring words
		-- simple:- converts title to lower case versions
		-- @@ matches operator
		WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
		-- @> contains operator for PostrgreSQL arrays
		AND (genres @> $2 OR $2 = '{}')
		ORDER BY %s %s, id ASC
	`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{title, pq.Array(genres)}
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	// close resultset before GetAll returns
	defer rows.Close()

	movies := []*Movie{}
	for rows.Next() {
		var movie Movie
		err := rows.Scan(
			&movie.ID,
			&movie.Title,
			&movie.Year,
			&movie.Runtime,
			pq.Array(&movie.Genres),
			&movie.Version,
			&movie.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		movies = append(movies, &movie)
	}
	// check for errors that might have occurred in the loop
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return movies, nil
}

func (m MovieModel) Get(id int) (*Movie, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id, title, year, runtime, genres, version 
		FROM movies
		WHERE id = $1
	`

	// nil struct to hold data returned by the query
	var movie Movie

	// context with a 3 second deadline
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	// ensures that resources associated with our context will always be released before Get() method returns
	// thereby preventing a memory leak
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&movie.ID,
		&movie.Title,
		&movie.Year,
		&movie.Runtime,
		pq.Array(&movie.Genres),
		&movie.Version,
	)

	// If no matching movie was found Scan() will return sql.ErrNoRows
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &movie, nil
}

func (m MovieModel) Update(movie *Movie) error {
	query := `
		UPDATE movies
		SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
		WHERE id = $5 AND version = $6
		RETURNING version
	`

	args := []any{
		movie.Title,
		movie.Year,
		movie.Runtime,
		pq.Array(movie.Genres),
		movie.ID,
		movie.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Prevent race condition through Optimistic locking
	// If no matching record could be found, we know movie
	// version has (changed or the record has been deleted) and we return a custom error
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (m MovieModel) Delete(id int) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
		DELETE FROM movies
		WHERE id = $1
	`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// if rows affected is zero
	// it means no movie with given id exists
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}
