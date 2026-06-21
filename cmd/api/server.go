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

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	shutdownError := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1) // make inicializa un objeto de tipo channel, slice o mapa
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		app.logger.Info("shutting down server", "signal", s.String())

		// Create a context with a 30-second timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Llama a Shutdown() en nuestro servidor, pasando el contexto que acabamos de crear.
		// Shutdown() devolverá nil si el cierre elegante fue exitoso, o un
		// error (que puede ocurrir debido a un problema al cerrar los oyentes, o
		// porque el cierre no se completó antes de que venciera la fecha límite de contexto de 30 segundos
		// golpe). Transmitimos este valor de retorno al canal ShutdownError.
		shutdownError <- srv.Shutdown(ctx)
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)
	// Llamar a Shutdown() en nuestro servidor hará que ListenAndServe() inmediatamente
	// devuelve un error http.ErrServerClosed. Entonces, si vemos este error, en realidad es un
	// algo bueno y una indicación de que ha comenzado el cierre elegante. Entonces comprobamos
	// específicamente para esto, solo devuelve el error si NO es http.ErrServerClosed.
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// De lo contrario, esperamos recibir el valor de retorno de Shutdown() en el
	// apagadoError canal. Si el valor de retorno es un error, sabemos que hubo un
	// problema con el apagado elegante y devolvemos el error.
	err = <-shutdownError
	if err != nil {
		return err
	}

	app.logger.Info("stopped server", "addr", srv.Addr)
	return nil
}
