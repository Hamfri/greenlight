package main

import (
	"fmt"
	"net/http"

	"golang.org/x/time/rate"
)

// will only recover panics that happen in the same goroutine that executed the recoverPanic middleware
func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// this function will always be run in the event
		// of a panic
		defer func() {
			// in-built recover() function to check if a panic occurred
			// if a panic did happen, recover will return the panic value else nil
			pv := recover()
			if pv != nil {
				// setting this header acts as trigger to make Go's HTTP server
				// automatically close the current connection after the response has been sent
				w.Header().Set("Connection", "close")
				// pv is of type any there we use %v to coerce it into an error
				app.internalServerErrorResponse(w, r, fmt.Errorf("%v", pv))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	// initialize a token based bucket rate limiter
	// min 2 requests per second
	// max 4 requests in a single burst
	limiter := rate.NewLimiter(2, 4)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//  allow requests if we have not exceeded max requests in a second
		if !limiter.Allow() {
			app.rateLimitExceededResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
