package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reminderapp/reminderbackend/internal/config"
	"github.com/reminderapp/reminderbackend/internal/push"
	"github.com/reminderapp/reminderbackend/internal/worker"
)

func main() {
	if err := config.LoadEnv(); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	credentialsPath := strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS"))
	if credentialsPath == "" {
		log.Fatal("FIREBASE_CREDENTIALS must point to a Firebase service account JSON file")
	}
	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		log.Fatalf("read Firebase service account credentials: %v", err)
	}
	fcm, err := push.NewClient(credentials)
	if err != nil {
		log.Fatalf("configure Firebase client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := pgxpool.New(ctx, databaseURL)
	cancel()
	if err != nil {
		log.Fatalf("configure database connection: %v", err)
	}
	defer db.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	err = db.Ping(ctx)
	cancel()
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Println("push notification worker started; scheduled daily at 00:01 UTC")
	if err := worker.RunDaily(runCtx, db, fcm); err != nil && err != context.Canceled {
		log.Fatalf("notification worker stopped: %v", err)
	}
}
