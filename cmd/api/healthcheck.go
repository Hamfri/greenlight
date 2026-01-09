package main

import (
	"net/http"
)

// all our handlers will be methods on the application struct
// this is an effective way to inject dependencies to our handlers
// without resorting to global variables or closures
func (app *application) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	env := envelope{
		"status": "available",
		"system_info": map[string]string{
			"environment": app.config.env,
			"version":     version,
		},
	}

	// simulates long running processes
	// when testing if graceful shutdown works as expected
	// time.Sleep(4 * time.Second)
	err := app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.internalServerErrorResponse(w, r, err)
	}
}
