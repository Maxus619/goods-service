package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"goods-service/internal/config"
	"goods-service/internal/repository"
	"goods-service/internal/service"
	transportHttp "goods-service/internal/transport/http"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	// Config
	cfg := config.MustLoad()

	// PostgreSQL
	pgPool := initPostgres(cfg)
	defer pgPool.Close()

	// Redis
	redisClient := initRedis(cfg)
	defer redisClient.Close()

	// ClickHouse
	clickhouseConn := initClickhouse(cfg)
	defer clickhouseConn.Close()

	// NATS
	natsConn := initNATS(cfg)
	defer natsConn.Close()

	// Repos
	postgresRepo := repository.NewPostgresRepository(pgPool)
	redisRepo := repository.NewRedisRepository(redisClient)
	clickhouseRepo := repository.NewClickhouseRepository(clickhouseConn)

	// NATS service
	natsSubscriber := service.NewNATSSubscriber(natsConn, clickhouseRepo)
	if err := natsSubscriber.Subscribe(); err != nil {
		log.Fatalf("failed to start NATS subscriber: %v", err)
	}

	// Service
	goodService := service.NewGoodService(postgresRepo, redisRepo, clickhouseRepo, natsConn)

	// Handler, Routes
	handler := transportHttp.NewHandler(goodService)
	router := transportHttp.NewRouter(handler)

	// HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.HttpPort,
		Handler: router,
	}

	// Start server
	go func() {
		log.Printf("starting server on port %s", cfg.HttpPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed to start: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exiting")
}

func initPostgres(cfg *config.Config) *pgxpool.Pool {
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DbUser,
		cfg.DbPassword,
		cfg.DbHost,
		cfg.DbPort,
		cfg.DbName,
	)

	log.Printf("connecting to PostgreSQL with connection string: %s",
		strings.Replace(connStr, cfg.DbPassword, "***", 1))

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}

	return pool
}

func initRedis(cfg *config.Config) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("unable to connect to Redis: %v", err)
	}

	return client
}

func initClickhouse(cfg *config.Config) clickhouse.Conn {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", cfg.ChHost, cfg.ChPort)},
		Auth: clickhouse.Auth{
			Database: "default",
		},
	})
	if err != nil {
		log.Fatalf("unable to connect to Clickhouse: %v", err)
	}

	return conn
}

func initNATS(cfg *config.Config) *nats.Conn {
	nc, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		log.Fatalf("unable to connect to NATS: %v", err)
	}

	return nc
}
