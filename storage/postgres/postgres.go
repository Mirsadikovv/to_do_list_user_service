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
	db              *pgxpool.Pool
	cfg             config.Config
	teacher         storage.TeacherRepoI
	support_teacher storage.SupportTeacherRepoI
	manager         storage.ManagerRepoI
	superadmin      storage.SuperadminRepoI
	administrator   storage.AdministratorRepoI
	branch          storage.BranchRepoI
	group           storage.GroupRepoI
	student         storage.StudentRepoI
	event           storage.EventRepoI
	eventRegistrate storage.EventRegistrateRepoI
	redis           storage.IRedisStorage
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

func (s *Store) Teacher() storage.TeacherRepoI {
	if s.teacher == nil {
		s.teacher = NewTeacherRepo(s.db)
	}
	return s.teacher
}

func (s *Store) SupportTeacher() storage.SupportTeacherRepoI {
	if s.support_teacher == nil {
		s.support_teacher = NewSupportTeacherRepo(s.db)
	}
	return s.support_teacher
}

func (s *Store) Manager() storage.ManagerRepoI {
	if s.manager == nil {
		s.manager = NewManagerRepo(s.db)
	}
	return s.manager
}

func (s *Store) Administrator() storage.AdministratorRepoI {
	if s.administrator == nil {
		s.administrator = NewAdministratorRepo(s.db)
	}
	return s.administrator
}

func (s *Store) Superadmin() storage.SuperadminRepoI {
	if s.superadmin == nil {
		s.superadmin = NewSuperadminRepo(s.db)
	}
	return s.superadmin
}

func (s *Store) Branch() storage.BranchRepoI {
	if s.branch == nil {
		s.branch = NewBranchRepo(s.db)
	}
	return s.branch
}

func (s *Store) Group() storage.GroupRepoI {
	if s.group == nil {
		s.group = NewGroupRepo(s.db)
	}
	return s.group
}

func (s *Store) Student() storage.StudentRepoI {
	if s.student == nil {
		s.student = NewStudentRepo(s.db)
	}
	return s.student
}

func (s *Store) Event() storage.EventRepoI {
	if s.event == nil {
		s.event = NewEventRepo(s.db)
	}
	return s.event
}

func (s *Store) EventRegistrate() storage.EventRegistrateRepoI {
	if s.eventRegistrate == nil {
		s.eventRegistrate = NewEventRegistrateRepo(s.db)
	}
	return s.eventRegistrate
}

func (s Store) Redis() storage.IRedisStorage {
	return redis.New(s.cfg)
}
