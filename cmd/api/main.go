package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const version = "1.0.0"

// env args passed via cmd flags on app start
type config struct {
	port int
	env  string
}

type application struct {
	config config
	logger *slog.Logger
}

func main() {
	var cfg config

	// read cmd-line flags and assign them to the config struct.
	// default port is 4000
	// default environment is "development"
	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.Parse()

	// structured logger that writes to the standard output stream
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// declare an instance of application struct
	// containing the config struct and the logger
	app := &application{
		config: cfg,
		logger: logger,
	}

	// declare an HTTP server which listens on the port provided in the config struct
	// use httpRouter as the Handler
	// writes any log messages to the structured logger at Error level.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	logger.Info("starting server", "addr", srv.Addr, "env", cfg.env)

	err := srv.ListenAndServe()
	logger.Error(err.Error())
	os.Exit(1)
}
