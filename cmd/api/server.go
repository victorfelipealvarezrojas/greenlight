package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	go func() {
		// Crea un canal de salida que transporta valores de os.Signal.
		quit := make(chan os.Signal, 1) // make inicializa un objeto de tipo channel, slice o mapa

		// signal.Notify() escuchar las señales SIGINT y SIGTERM entrantes y
		// retransmitirlos al canal de salida. Cualquier otra señal no será captada por
		// signal.Notify() y conservará su comportamiento predeterminado.
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		s := <-quit // El operador de canal de recepción (<-) bloquea la ejecución hasta que se recibe una señal en el canal quit.

		app.logger.Info("caught signal", "signal", s.String())

		os.Exit(0)

	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

	return srv.ListenAndServe()
}
