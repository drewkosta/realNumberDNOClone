package boot

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func Serve(app *App, port string, handler http.Handler, onShutdown func()) {
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		app.Logger.Info("Service starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.Logger.Error("Service failed", "error", err)
		}
	}()

	<-ctx.Done()
	app.Logger.Info("Shutting down...")

	if onShutdown != nil {
		onShutdown()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)

	app.Logger.Info("Service stopped")
}
