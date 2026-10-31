package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	envFile := flag.String("env", ".env", "configuration file; environment variables take precedence")
	flag.Parse()
	cfg, err := loadConfig(*envFile, os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	app := newApp(cfg, &http.Client{Timeout: 75 * time.Second})
	server := &http.Server{
		Addr: net.JoinHostPort(cfg.Host, cfg.Port), Handler: app,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 95 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- server.ListenAndServe() }()
	log.Printf("Find Filament listening on %s (web interface embedded in executable)", server.Addr)
	select {
	case err := <-finished:
		if err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-stop.Done():
		ctx, release := context.WithTimeout(context.Background(), 90*time.Second)
		defer release()
		if err := server.Shutdown(ctx); err != nil {
			_ = server.Close()
			log.Printf("Shutdown: %v", err)
		}
	}
}
