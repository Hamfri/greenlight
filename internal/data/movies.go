package data

import "time"

type Movie struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"-"` // -  omits the field entirely from json response
	Title     string    `json:"title"`
	Year      int       `json:"year,omitzero"`    // omitzero introduced in go 1.24 removes the field if it has the zero value of the type
	Runtime   Runtime   `json:"runtime,omitzero"` // Movie runtime in minutes
	Genres    []string  `json:"genres,omitempty"` // Slice of genres for the movie (romance, comedy, etc.) omitempty useful for slices & maps
	Version   int       `json:"version"`          // Version starts at one and will be incremented each time movie information is updated
}
