package main

import (
	"errors"
	"fmt"
	"greenlight/internal/data"
	"greenlight/internal/validator"
	"net/http"
	"strings"
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
	if !app.config.limiter.enabled {
		return next
	}

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
			clients[ip] = &client{limiter: rate.NewLimiter(rate.Limit(app.config.limiter.rps), app.config.limiter.burst)}
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

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// indicates to caches that the response may vary
		// depending on the value of the Authorization header in the request
		w.Header().Add("Vary", "Authorization")

		authorizationHeader := r.Header.Get("Authorization")

		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		token := headerParts[1]
		v := validator.New()

		if data.ValidatePlainTextToken(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		user, err := app.models.Users.GetUserByToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.internalServerErrorResponse(w, r, err)
			}
			return
		}

		r = app.contextSetUser(r, user)
		next.ServeHTTP(w, r)
	})
}

// we HandlerFunc so that we can wrap the handlers directly
func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)

	})
}

// checks that a user is both authenticated and activated
// flow requireAuthenticatedUser -> requireActivatedUser
// with this flow we only check if a user is activated when we have real user
func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	}

	return app.requireAuthenticatedUser(fn)
}

// flow requireAuthenticatedUser -> requireActivatedUser -> requirePermission
func (app *application) requirePermission(code data.Permission, next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		permissions, err := app.models.Permissions.GetUserPermissions(user.ID)
		if err != nil {
			app.internalServerErrorResponse(w, r, err)
			return
		}

		if !permissions.Includes(code) {
			app.notPermittedResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	}

	return app.requireActivatedUser(fn)
}
