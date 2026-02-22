package main

import (
	"context"
	"freedom_bitrix/internal/bitrix"
	"freedom_bitrix/internal/config"
	"freedom_bitrix/internal/repo"
	"freedom_bitrix/internal/server"
	"freedom_bitrix/internal/syncer"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	stateKey = "deals_sync"
	overlap  = 10 * time.Minute
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	mode := "delta"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	bx := bitrix.NewClient(cfg.BitrixWebhookBaseURL)

	pool, err := pgxpool.New(runCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	repository := repo.NewDealsRepository(pool)
	if err := repository.Migrate(runCtx); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	syncService := syncer.NewService(bx, repository, stateKey, overlap)
	httpServer := server.New(repository, bx)

	switch mode {
	case "full":
		if err := syncService.FullSync(runCtx); err != nil {
			log.Fatal(err)
		}
	case "delta":
		if err := syncService.DeltaSync(runCtx); err != nil {
			log.Fatal(err)
		}
	case "serve-delta":
		if err := syncService.DeltaSync(runCtx); err != nil {
			log.Fatal(err)
		}
		if err := httpServer.Start(":8080"); err != nil {
			log.Fatal(err)
		}
		return
	case "serve":
		if err := httpServer.Start(":8080"); err != nil {
			log.Fatal(err)
		}
		return
	default:
		log.Fatalf("unknown mode: %s (use: full | delta | serve | serve-delta)", mode)
	}

	log.Println("DONE")
}
