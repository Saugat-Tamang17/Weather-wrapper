package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Saugat-Tamang17/weather-wrapper/internal/config"
	"github.com/Saugat-Tamang17/weather-wrapper/internal/handler"
	"github.com/Saugat-Tamang17/weather-wrapper/internal/weather"
)

func main() {
	// 1. Load configuration (port, API keys, etc.)
	cfg := config.Load()

	// 2. Create weather client (talks to external API) //
	weatherClient := weather.NewClient(cfg.APIURL, cfg.CacheTime)

	// 3. Create HTTP handler (depends on client)
	weatherHandler := handler.New(weatherClient)

	// 4. Create router
	mux := http.NewServeMux()

	// 5. Register routes
	mux.Handle("/weather", weatherHandler)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "seems okay")
	})

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	go func() {
		log.Printf("Server starting on port :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed:%v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 6. Start server
	log.Printf("Server starting on port :%s", cfg.Port)

	err := http.ListenAndServe(":"+cfg.Port, mux)
	if err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
