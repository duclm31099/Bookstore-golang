package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/duclm99/bookstore-backend-v2/internal/bootstrap"
)

func main() {
	app, cleanup, err := bootstrap.InitializeAPIApp()
	if err != nil {
		log.Fatalf("failed to initialize api app: %v", err)
	}
	defer cleanup()

	go func() {
		if err := app.Run(); err != nil {
			log.Fatalf("api app stopped with error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	if err := app.Shutdown(); err != nil {
		log.Fatalf("failed to shutdown api app: %v", err)
	}
}
