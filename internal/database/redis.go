package database

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type RedisDB struct {
	Client *redis.Client
}

func NewRedisDB(url, password string, db int, logger *zap.Logger) (*RedisDB, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Si se proporciona una contraseña separada, usarla
	if password != "" {
		opt.Password = password
	}
	opt.DB = db

	client := redis.NewClient(opt)

	// Verificar conexión
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	logger.Info("Redis connection established",
		zap.String("addr", opt.Addr),
		zap.Int("db", db),
	)

	return &RedisDB{Client: client}, nil
}

func (r *RedisDB) Close() error {
	return r.Client.Close()
}

func (r *RedisDB) Ping(ctx context.Context) error {
	return r.Client.Ping(ctx).Err()
}

// GetStats retorna estadísticas básicas de Redis
func (r *RedisDB) GetStats(ctx context.Context) (string, error) {
	info := r.Client.Info(ctx, "stats")
	return info.Result()
}
