package data

import (
	"greenlight/internal/validator"
	"time"
)

type Movie struct {
	// since we are using BIGINT on postgreSQL it's better to use int64 here to avoid errors
	// Go infers `int` type from os which would be int64 on 64 bit systems and int 32 on 32 bit systems
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"` // -  omits the field entirely from json response
	Title     string    `json:"title"`
	// here we cap int to int32 so that it matches with postgres Integer type and avoid introducing errors when we exceed int32's upper and lower limits
	// or integer overflow errors see >>> https://go.dev/ref/spec#Integer_overflow
	Year    int32    `json:"year,omitzero"`    // omitzero introduced in go 1.24 removes the field if it has the zero value of the type
	Runtime Runtime  `json:"runtime,omitzero"` // Movie runtime in minutes
	Genres  []string `json:"genres,omitempty"` // Slice of genres for the movie (romance, comedy, etc.) omitempty useful for slices & maps
	Version int32    `json:"version"`          // Version starts at one and will be incremented each time movie information is updated
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
