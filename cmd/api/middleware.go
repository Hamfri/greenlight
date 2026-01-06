package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/tomasen/realip"
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
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client) // key = client ip
	)

	// goroutine to remove old entries from clients map
	// runs once every minute
	go func() {
		for {
			time.Sleep(time.Minute)

			// lock mutex to prevent any rate limiter checks from happening
			// during cleanup
			mu.Lock()

			// delete client from map if they haven been seen within the last three minutes
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := realip.FromRequest(r)

		// lock mutex to prevent map data corruption as a result of race conditions
		mu.Lock()
		if _, found := clients[ip]; !found {
			// initialize a token based bucket rate limiter
			// min 2 requests per second
			// max 4 requests in a single burst
			clients[ip] = &client{limiter: rate.NewLimiter(2, 4)}
		}

		// update last seen time for the client
		clients[ip].lastSeen = time.Now()

		//  allow requests if we have not exceeded max requests in a second
		if !clients[ip].limiter.Allow() {
			mu.Unlock()
			app.rateLimitExceededResponse(w, r)
			return
		}

		// don't use defer to unlock mutex here
		// that would mean that we have to wait untill all
		// handlers downstream of this middleware have returned
		mu.Unlock()

		next.ServeHTTP(w, r)
	})
}
