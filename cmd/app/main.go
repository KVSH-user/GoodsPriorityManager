package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"hezzl_test/internal/config"
	"hezzl_test/internal/http-server/handlers/goods"
	"hezzl_test/internal/http-server/middleware/logger"
	natss "hezzl_test/internal/nats"
	"hezzl_test/internal/storage/clickhouse"
	"hezzl_test/internal/storage/postgres"
	"log/slog"
	"net/http"
	"os"
)

const (
	envDev  = "dev"
	envProd = "prod"
)

func main() {
	cfg := config.MustLoad()

	log := SetupLogger(cfg.Env)

	log.Info("App started", slog.String("env", cfg.Env))
	log.Debug("Debugging started")

	storage, err := postgres.New(
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.DBName,
	)
	if err != nil {
		log.Error("failed to init storage: ", err)
		os.Exit(1)
	}

	log.Info("Loaded configuration", slog.Any("config", cfg))

	chDB, err := clickhouse.SetupClickHouseConnection(
		cfg.ClickHouse.Host,
		cfg.ClickHouse.Port,
		cfg.ClickHouse.User,
		cfg.ClickHouse.Password,
		cfg.ClickHouse.DBName,
	)
	if err != nil {
		log.Error("Error initializing ClickHouse connection: ", err)
		os.Exit(1)
	}

	clickTable := clickhouse.CreateTableClickHouse(chDB)
	if err != nil {
		log.Error("failed create ClickHouse table: ", err)
	}
	_ = clickTable
	defer chDB.Close()

	natsConn, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Error("Error connecting to NATS: ", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	err = natss.SubscribeToNATSEvents(natsConn, chDB)

	go clickhouse.StartFlusher()

	log.Info("storage successfully initialized")

	router := chi.NewRouter()

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "PUT", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	router.Use(middleware.RequestID)
	router.Use(middleware.Logger)
	router.Use(logger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)
	router.Use(corsHandler.Handler)

	router.Post("/good/create/{projectId}", goods.Create(log, storage, natsConn))
	router.Patch("/good/update/{id}/{projectId}", goods.Update(log, storage, redisClient, natsConn))
	router.Delete("/good/remove/{id}/{projectId}", goods.Remove(log, storage, redisClient, natsConn))
	router.Get("/goods/list", goods.List(log, storage, redisClient))
	router.Patch("/good/reprioritize/{id}/{projectId}", goods.Reprioritize(log, storage, redisClient, natsConn))

	log.Info("starting server", slog.String("address", cfg.HTTPServer.Address))

	srv := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Error("failed to start server")
	}

	log.Error("server stopped")
}

func SetupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)

	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}
