package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/peniakoff/weles/internal/app"
)

func main() {
	ctx := context.Background()
	deps, err := app.Build(ctx)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}

	srv := &http.Server{
		Addr:              deps.Env.ListenAddr,
		Handler:           deps.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    8 << 10,
	}

	go func() {
		deps.Logger.Info("weles listening", "addr", deps.Env.ListenAddr, "notifier", deps.Env.Notifier, "turnstile", deps.Env.TurnstileMode)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			deps.Logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
