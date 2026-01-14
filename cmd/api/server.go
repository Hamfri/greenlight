package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	// declare an HTTP server that listens on the port provided in the config struct
	// use httpRouter as the Handler
	// write any log messages to the structured logger at Error level.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	// receives errors returned by the graceful Shutdown() function
	shutDownError := make(chan error)

	// runs for the lifetime of our application
	go func() {
		// quit is a buffered channel with size 1
		// not defining size means it's unbuffered
		// used to carry os.Signal values
		// if it was unbuffered a signal could be missed
		// if our channel was not ready to receive
		// at the exact moment a signal is sent
		quit := make(chan os.Signal, 1)

		// listen to SIGINT and SIGTERM signals and
		// relay them to quit channel.
		// All other signals SIGQUIT, SIGKILL, ... retain their behavior.
		// signal.Notify doesn't wait for a receiver to be available
		// when sending a signal to quit channel
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Read signal from quit channel.
		// code will block until a signal is received
		s := <-quit

		// s.String() includes signal name in the log entry attributes
		app.logger.Info("shutting down server", "signal", s.String())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Graceful shutdown
		// shutdown will return nil if graceful Shutdown() was successful, or an
		// error (if there was a problem closing the listeners or 30 second context deadline is hit)
		// we relay this return value to the shutdownError channel
		shutDownError <- srv.Shutdown(ctx) // instead of os.Exit(0)
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

	// calling shutdown on srv will return err `http.ErrServerClosed`
	// to indicate graceful Shutdown() has started.
	// otherwise we return the error
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// wait for return value from Shutdown()
	// if it's an error there was a problem with graceful shutdown
	if err = <-shutDownError; err != nil {
		return err
	}

	// Wait() prevents serve from returning to main()
	// until Waitgroup's counter is zero.
	// that ensures that all background processes run to completion
	app.logger.Info("completing background tasks", "addr", srv.Addr)
	app.wg.Wait()

	// if graceful shutdown was successful log message
	app.logger.Info("stopped server", "addr", srv.Addr)

	return nil
}
