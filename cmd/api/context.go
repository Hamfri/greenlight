package main

import (
	"context"
	"greenlight/internal/data"
	"net/http"
)

// custom `string` type to help prevent key collisions
type contextKey string

// convert string `"user"` to contextKey type
// we will be using this constant as the key for getting user information
// in the request context
const userContextKey = contextKey("user")

func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
	// commented out code is prone to key collisions from 3rd party packages
	// that could be storing data by the same key
	// ctx := context.WithValue(r.Context(), "user", user)
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func (app *application) contextGetUser(r *http.Request) *data.User {
	// request context values are stored with type `any`.
	// To retrieve `user` value from context
	// we must assert that it contains a pointer to `*data.User` hence the `ok`
	// `ok` is set to true if conversion was successful
	// we use a pointer to avoid copying large structs and because mutations are shared across handlers and middleware
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		panic("missing user value in request context")
	}

	return user
}
