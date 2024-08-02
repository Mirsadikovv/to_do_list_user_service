package postgres

import (
	"context"
	"fmt"
	"go_user_service/config"
	"go_user_service/storage"
	"go_user_service/storage/redis"
	"log"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Store struct {
	db            *pgxpool.Pool
	cfg           config.Config
	administrator storage.AdminRepoI
	user          storage.UserRepoI
	redis         storage.IRedisStorage
}

func NewPostgres(ctx context.Context, cfg config.Config, redis storage.IRedisStorage) (storage.StorageI, error) {
	config, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDatabase,
	))
	if err != nil {
		return nil, err
	}

	config.MaxConns = cfg.PostgresMaxConnections

	pool, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return &Store{
		db:    pool,
		redis: redis,
	}, err
}

func (s *Store) CloseDB() {
	s.db.Close()
}

func (l *Store) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
	args := make([]interface{}, 0, len(data)+2) // making space for arguments + level + msg
	args = append(args, level, msg)
	for k, v := range data {
		args = append(args, fmt.Sprintf("%s=%v", k, v))
	}
	log.Println(args...)
}

func (s *Store) Admin() storage.AdminRepoI {
	if s.administrator == nil {
		s.administrator = NewAdminRepo(s.db)
	}
	return s.administrator
}

func (s *Store) User() storage.UserRepoI {
	if s.user == nil {
		s.user = NewUserRepo(s.db)
	}
	return s.user
}

func (s Store) Redis() storage.IRedisStorage {
	return redis.New(s.cfg)
}
