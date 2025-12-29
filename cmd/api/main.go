package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"greenlight/internal/data"
	"log/slog"
	"net/http"
	"os"
	"time"

	// import pq driver so that it can register itself with the database/sql package
	// `_` is an alias to stop the compiler from complaining that package isn't being used
	_ "github.com/lib/pq"
)

const version = "1.0.0"

// env args passed via cmd flags on app start
type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
}

type application struct {
	config config
	logger *slog.Logger
	model  data.Models
}

func main() {
	var cfg config

	// read cmd-line flags and assign them to the config struct.
	// default port is 4000
	// default environment is "development"
	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	// os.Getenv("GREENLIGHT_DB_DSN") // read from environment variables
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")

	// DB connection pool settings from cmd-line flags
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")
	flag.Parse()

	// structured logger that writes to the standard output stream
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// open a database connection pool
	// in the event of an error we log the error and exit the application immediately
	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// close the connection pool before main() function exits
	defer db.Close()
	logger.Info("database connection pool established")

	// declare an instance of application struct
	// containing the config struct and the logger
	app := &application{
		config: cfg,
		logger: logger,
		model:  data.NewModels(db),
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

	err = srv.ListenAndServe()
	logger.Error(err.Error())
	os.Exit(1)
}

func openDB(cfg config) (*sql.DB, error) {
	// create an empty connection pool
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	// max number of open connections (in-use + idle)
	// a value less than or equal to zero means there is no limit
	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	// a value less than or equal to zero means there is no limit
	db.SetMaxIdleConns(cfg.db.maxIdleConns)
	// // a value less than or equal to zero means connections are not closed due to their idle time
	db.SetConnMaxIdleTime(cfg.db.maxIdleTime)

	// create context with a 5-second timeout deadline
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// establish a new connection to the database, passing in the context
	// If connection couldn't be successfully established within 5 seconds then this will return an error
	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}

	// sql.DB connection pool
	return db, nil
}
