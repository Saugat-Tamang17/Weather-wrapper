package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Saugat-Tamang17/weather-wrapper/internal/config"
)

func main() {
	cfg := config.Load()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "seems okay")
	})

	log.Printf("Starting the server on the port :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("server failed:%v", err)
	}
}
