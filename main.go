package main

import (
	"log"
	"net/http"

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
	logger.Info("Storage initialized")

	r := relay.NewRelay(db)
	logger.Info("Starting Relay")
	go r.Run()

	http.HandleFunc("/", func(w http.ResponseWriter, rq *http.Request) {
		r.Serve(w, rq)
	})

	if conf.Auth.NoAuth {
		err = http.ListenAndServe(conf.Relay.GetAddress(), nil)
		if err != nil {
			logger.Fatal("ListenAndServe: ", err)
		}
	} else {
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
}
