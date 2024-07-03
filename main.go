package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"sharegap.net/nostrodomo/config"
	"sharegap.net/nostrodomo/logger"
	"sharegap.net/nostrodomo/relay"
	"sharegap.net/nostrodomo/storage"
)

func main() {

	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	logger.InitLogger(conf.Logging)

	db, err := storage.InitStorage(&conf.Storage)
	if err != nil {
		logger.Fatal("Failed to initialize storage:", err)
	}
	defer db.Disconnect()
	logger.Info("Storage initialized")

	r := relay.NewRelay(db, &conf.WebSocket)

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		log.Printf("Received signal: %s, shutting down...", sig)
		cancel()
	}()

	// Start relay
	go r.Run()

	http.HandleFunc("/", func(w http.ResponseWriter, rq *http.Request) {
		r.Serve(w, rq)
	})


	// Start the server in a goroutine
	go func() {
		logger.Info("Starting server on ", conf.Relay.GetAddress())
		if conf.Auth.NoAuth {
			err = http.ListenAndServe(conf.Relay.GetAddress(), nil)
			if err != nil {
				logger.Fatal("ListenAndServe: ", err)
			}
		} else {
			logger.Info("Starting TLS Server")
			err = http.ListenAndServeTLS(
				conf.Relay.GetAddress(),
				conf.Auth.Cert,
				conf.Auth.Key,
				nil,
			)
			if err != nil {
				logger.Fatal("ListenAndServeTLS: ", err)
			}
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	r.Shutdown()
	log.Println("Nostrodomo shut down gracefully")

}
