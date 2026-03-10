package main

import (
	"log"
	"net/http"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/env"
	httptransport "github.com/xanderbilla/bi8s-go/internal/http"
)

func main() {

	cfg := app.Config{
		Addr: ":8080",
		Env:  env.GetString("ENV", "prod"),
	}

	app := &app.Application{
		Config: cfg,
	}

	mux := httptransport.Mount(app)

	srv := http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  time.Minute,
	}

	log.Printf("server started on %s", cfg.Addr)

	log.Fatal(srv.ListenAndServe())
}
