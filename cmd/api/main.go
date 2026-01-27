package main

import (
	"context"
	"database/sql"
	"expvar"
	"flag"
	"fmt"
	"greenlight/internal/data"
	"greenlight/internal/mailer"
	"greenlight/internal/vcs"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	// import pq driver so that it can register itself with the database/sql package
	// `_` is an alias to stop the compiler from complaining that package isn't being used
	_ "github.com/lib/pq"
)

var (
	version = vcs.Version()
)

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
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config config
	logger *slog.Logger
	models data.Models
	mailer *mailer.Mailer
	wg     *sync.WaitGroup
}

func main() {
	var cfg config

	// read cmd-line flags and assign them to the config struct.
	// default port is 4000
	// default environment is "development"
	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	// os.Getenv("GREENLIGHT_DB_DSN") // read from environment variables
	flag.StringVar(&cfg.db.dsn, "db-dsn", "", "PostgreSQL DSN")

	// DB connection pool settings from cmd-line flags
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

	// Rate limiter settings from cmd-line flags
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	// smtp settings
	flag.StringVar(&cfg.smtp.host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 25, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "fake-username", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "fake-pwd", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "fake-sender", "SMTP sender")

	flag.Func("trusted-cors", "Trusted cross origin resource sharing", func(val string) error {
		// strings.Fields splits space separated strings into a slice
		// slice is empty if trusted-cors is not provided or trusted-cors = ""
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.Parse()

	if *displayVersion {
		fmt.Printf("api version:\t%s\n", version)
		os.Exit(0)
	}

	// structured logger that writes to the standard output stream
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// open a database connection pool
	// in the event of an error we log the error and exit the application immediately
	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		// non-zero code indicates an error
		os.Exit(1)
	}

	// close the connection pool before main() function exits
	defer db.Close()

	logger.Info("database connection pool established")

	mailer, err := mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// app metrics
	expvar.NewString("version").Set(version)

	// number of active goroutines
	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	// database connection pool statistics
	expvar.Publish("database", expvar.Func(func() any {
		return db.Stats()
	}))

	// current unix timestamp
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))

	// declare an instance of application struct
	// containing the config struct and the logger
	app := &application{
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer,
		wg:     &sync.WaitGroup{},
	}

	if err = app.serve(); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
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
