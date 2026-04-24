package main

import (
	"fmt"
	"log"
	"os"

	"github.com/duclm99/bookstore-backend-v2/internal/platform/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate [up|down|version]")
	}

	cfg := config.MustLoad()
	m, err := migrate.New(
		"file://"+cfg.DB.MigrationDir,
		cfg.DB.DSN(),
	)
	if err != nil {
		log.Fatal(err)
	}

	switch os.Args[1] {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatal(err)
		}
	case "down":
		if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
			log.Fatal(err)
		}
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}
