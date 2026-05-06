package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	log.Println("Shutdown Signal has been Received (CTRL+C or docker stop), Draining the inflight Request ...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Forced Shutdown :%v", err)
	}
	log.Println("Server Exited Properly or Cleanly I suppose ")
}
