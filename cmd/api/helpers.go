package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

func (app *application) readIdParam(r *http.Request) (int, error) {
	// any interpolated URL parameters are stored in the request context
	// We can retrieve a slice containing this parameters from `ParamsFromContext`
	params := httprouter.ParamsFromContext(r.Context())

	id, err := strconv.Atoi(params.ByName("id"))
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}

	return id, nil
}

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	// js is a []byte slice containing encoded JSON
	// "" means no prefix for lines and \t indicates that we add tab for each element
	// json.MarshalIndent uses slightly more memory in comparison to json.Marshal avoid it in memory constrained environments
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	// append newline to JSON. This is just a small nicety to make it easier to
	// view in terminal applications
	js = append(js, '\n')
	for key, values := range headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	return nil
}

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// limit the size of the request body to 1_048_576 bytes (1MB)
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	// Decode modifies input therefore it must be a non-nil pointer
	// json.NewDecoder is better than json.Unmarshal because it reads
	// incrementally from io.reader unlike Unmarshal which reads once from io.reader and loads everything to memory (High memory usage)
	dec := json.NewDecoder(r.Body)
	// ensures that we only decode known fields
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			// handles wrong data formats eg xml
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			// can be triggered by passing invalid Content-Length: 10
			return errors.New("body contains badly formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				// handles wrong field types, string provided as int
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			// handles wrong json body types eg if we expect an object {} and we receive an array [] instead
			return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			// handles empty json body
			return errors.New("body must not be empty")
		// see github issue https://github.com/golang/go/issues/29035
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)
		case errors.As(err, &invalidUnmarshalError):
			// this is an unexpected programmer error hence the reason why we use panic
			// can be triggered in code by us passing a non-nil pointer to json.NewDecoder
			panic(err)
		default:
			return err
		}
	}

	// call decode again, using a pointer to an empty anonymous struct as the
	// destination. If request body contained a single JSON value this will
	// return io.EOF. Anything else means there was additional data after the request body
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}
