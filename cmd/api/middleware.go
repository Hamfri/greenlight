package main

import (
	"fmt"
	"net/http"
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
