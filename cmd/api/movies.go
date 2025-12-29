package main

import (
	"fmt"
	"greenlight/internal/data"
	"greenlight/internal/validator"
	"net/http"
	"time"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	// all attributes must be exported i.e `be public`
	// so that they are visble to encoding/json package
	// struct tags must match the incoming json request key
	var input struct {
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		Runtime data.Runtime `json:"runtime"`
		Genres  []string     `json:"genres"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// primitive types such as int, float, bool, struct and array must use `&` address operator if we need a pointer
	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}

	// since map, channel, interface and functions are implemented as pointer types we don't require `&` addresss operator if we need a pointer
	// since we intend to reuse the validator in other handlers it's better to
	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// write to response fmt.Fprintf(w, "%+v\n", input)
	fmt.Fprintf(w, "%+v\n", input)
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIdParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	movies := data.Movie{
		ID:        int64(id),
		CreatedAt: time.Now(),
		Title:     "",
		Year:      2025,
		Runtime:   60,
		Genres:    []string{"romance"},
		Version:   1,
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": movies}, nil)
	if err != nil {
		app.internalServerErrorResponse(w, r, err)
	}
}
