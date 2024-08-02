package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go_user_service/genproto/user_service"
	"go_user_service/pkg"
	"go_user_service/pkg/hash"
	"go_user_service/pkg/logger"
	"go_user_service/storage"
	"log"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

type userRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) storage.UserRepoI {
	return &userRepo{
		db: db,
	}
}

func generateUserLogin(db *pgxpool.Pool, ctx context.Context) (string, error) {
	var nextVal int
	err := db.QueryRow(ctx, "SELECT nextval('user_external_id_seq')").Scan(&nextVal)
	if err != nil {
		return "", err
	}
	userLogin := "S" + fmt.Sprintf("%05d", nextVal)
	return userLogin, nil
}

func (c *userRepo) Create(ctx context.Context, req *user_service.CreateUser) (*user_service.GetUser, error) {
	var finished_at sql.NullString
	if req.FinishedAt == "" {
		finished_at = sql.NullString{Valid: false}
	} else {
		finished_at = sql.NullString{String: req.FinishedAt, Valid: true}
	}
	id := uuid.NewString()
	pasword, err := hash.HashPassword(req.UserPassword)
	if err != nil {
		log.Println("error while hashing password", err)
	}

	userLogin, err := generateUserLogin(c.db, ctx)
	if err != nil {
		log.Println("error while generating login", err)
	}

	comtag, err := c.db.Exec(ctx, `
		INSERT INTO users (
			id,
			group_id,
			user_login,
			birthday,
			gender,
			fullname,
			email,
			phone,
			user_password,
			paid_sum,
			started_at,
			finished_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12
		)`,
		id,
		req.GroupId,
		userLogin,
		req.Birthday,
		req.Gender,
		req.Fullname,
		req.Email,
		req.Phone,
		pasword,
		req.PaidSum,
		req.StartedAt,
		finished_at)
	if err != nil {
		log.Println("error while creating user", comtag)
		return nil, err
	}

	user, err := c.GetById(ctx, &user_service.UserPrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting user by id")
		return nil, err
	}
	return user, nil
}

func (c *userRepo) Update(ctx context.Context, req *user_service.UpdateUser) (*user_service.GetUser, error) {
	var finished_at sql.NullString
	if req.FinishedAt == "" {
		finished_at = sql.NullString{Valid: false}
	} else {
		finished_at = sql.NullString{String: req.FinishedAt, Valid: true}
	}
	_, err := c.db.Exec(ctx, `
		UPDATE users SET
		group_id = $1,
		birthday = $2,
		gender = $3,
		fullname = $4,
		email = $5,
		phone = $6,
		paid_sum = $7,
		started_at = $8,
		finished_at = $9,
		updated_at = NOW()
		WHERE id = $10
		`,
		req.GroupId,
		req.Birthday,
		req.Gender,
		req.Fullname,
		req.Email,
		req.Phone,
		req.PaidSum,
		req.StartedAt,
		finished_at,
		req.Id)
	if err != nil {
		log.Println("error while updating user")
		return nil, err
	}

	user, err := c.GetById(ctx, &user_service.UserPrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting user by id")
		return nil, err
	}
	return user, nil
}

func (c *userRepo) GetAll(ctx context.Context, req *user_service.GetListUserRequest) (*user_service.GetListUserResponse, error) {
	users := user_service.GetListUserResponse{}

	var (
		created_at  sql.NullString
		updated_at  sql.NullString
		started_at  sql.NullString
		finished_at sql.NullString
	)
	filter_by_name := ""
	offest := (req.Offset - 1) * req.Limit
	if req.Search != "" {
		filter_by_name = fmt.Sprintf(`AND fullname ILIKE '%%%v%%'`, req.Search)
	}
	query := `SELECT
				id,
				group_id,
				user_login,
				birthday,
				gender,
				fullname,
				email,
				phone,
				paid_sum,
				started_at,
				finished_at,
				created_at,
				updated_at
			FROM users
			WHERE TRUE AND deleted_at is null ` + filter_by_name + `
			OFFSET $1 LIMIT $2
`
	rows, err := c.db.Query(ctx, query, offest, req.Limit)

	if err != nil {
		log.Println("error while getting all users")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			user user_service.GetUser
		)
		if err = rows.Scan(
			&user.Id,
			&user.GroupId,
			&user.UserLogin,
			&user.Birthday,
			&user.Gender,
			&user.Fullname,
			&user.Email,
			&user.Phone,
			&user.PaidSum,
			&started_at,
			&finished_at,
			&created_at,
			&updated_at,
		); err != nil {
			return &users, err
		}
		user.StartedAt = pkg.NullStringToString(started_at)
		user.FinishedAt = pkg.NullStringToString(finished_at)
		user.CreatedAt = pkg.NullStringToString(created_at)
		user.UpdatedAt = pkg.NullStringToString(updated_at)

		users.Users = append(users.Users, &user)
	}

	err = c.db.QueryRow(ctx, `SELECT count(*) from users WHERE TRUE AND deleted_at is null `+filter_by_name+``).Scan(&users.Count)
	if err != nil {
		return &users, err
	}

	return &users, nil
}

func (c *userRepo) GetById(ctx context.Context, id *user_service.UserPrimaryKey) (*user_service.GetUser, error) {
	var (
		user        user_service.GetUser
		created_at  sql.NullString
		updated_at  sql.NullString
		started_at  sql.NullString
		finished_at sql.NullString
	)

	query := `SELECT
				id,
				group_id,
				user_login,
				birthday,
				gender,
				fullname,
				email,
				phone,
				paid_sum,
				started_at,
				finished_at,
				created_at,
				updated_at
			FROM users
			WHERE id = $1 AND deleted_at IS NULL`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&user.Id,
		&user.GroupId,
		&user.UserLogin,
		&user.Birthday,
		&user.Gender,
		&user.Fullname,
		&user.Email,
		&user.Phone,
		&user.PaidSum,
		&started_at,
		&finished_at,
		&created_at,
		&updated_at); err != nil {
		return &user, err
	}
	user.StartedAt = pkg.NullStringToString(started_at)
	user.FinishedAt = pkg.NullStringToString(finished_at)
	user.CreatedAt = pkg.NullStringToString(created_at)
	user.UpdatedAt = pkg.NullStringToString(updated_at)

	return &user, nil
}

func (c *userRepo) Delete(ctx context.Context, id *user_service.UserPrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE users SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}

func (c *userRepo) Check(ctx context.Context, id *user_service.UserPrimaryKey) (*user_service.CheckUserResp, error) {
	query := `SELECT EXISTS (
                SELECT 1
                FROM users
                WHERE id = $1 AND deleted_at IS NULL
            )`

	var exists bool
	err := c.db.QueryRow(ctx, query, id.Id).Scan(&exists)
	if err != nil {
		return nil, err
	}

	resp := &user_service.CheckUserResp{
		Check: exists,
	}

	return resp, nil
}

func (c *userRepo) ChangePassword(ctx context.Context, pass *user_service.UserChangePassword) (*user_service.UserChangePasswordResp, error) {
	var hashedPass string
	var resp user_service.UserChangePasswordResp
	query := `SELECT user_password
	FROM users
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query,
		pass.UserLogin,
	).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("incorrect login")
		}
		log.Println("failed to get user password from database", logger.Error(err))
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(pass.OldPassword))
	if err != nil {
		return nil, errors.New("password mismatch")
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(pass.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Println("failed to generate user new password", logger.Error(err))
		return nil, err
	}

	query = `UPDATE users SET 
		user_password = $1, 
		updated_at = NOW() 
	WHERE user_login = $2 AND deleted_at is null`

	_, err = c.db.Exec(ctx, query, newHashedPassword, pass.UserLogin)
	if err != nil {
		log.Println("failed to change user password in database", logger.Error(err))
		return nil, err
	}
	resp.Comment = "Password changed successfully"

	return &resp, nil
}

func (c *userRepo) GetByLogin(ctx context.Context, login string) (*user_service.GetUserByLogin, error) {
	var (
		user        user_service.GetUserByLogin
		birthday    sql.NullString
		started_at  sql.NullString
		finished_at sql.NullString
		created_at  sql.NullString
		updated_at  sql.NullString
	)

	query := `SELECT 
		id, 
		branch_id,
		user_login,
		birthday, 
		gender,
		fullname,
		email,
		phone,
		user_password,
		paid_sum,
		started_at,
		finished_at,
		created_at, 
		updated_at
		FROM users WHERE user_login = $1 AND deleted_at is null`

	row := c.db.QueryRow(ctx, query, login)

	err := row.Scan(
		&user.Id,
		&user.GroupId,
		&user.UserLogin,
		&birthday,
		&user.Gender,
		&user.Fullname,
		&user.Email,
		&user.Phone,
		&user.UserPassword,
		&user.PaidSum,
		&started_at,
		&finished_at,
		&created_at,
		&updated_at,
	)

	if err != nil {
		log.Println("failed to scan user by LOGIN from database", logger.Error(err))
		return &user_service.GetUserByLogin{}, err
	}

	user.Birthday = pkg.NullStringToString(birthday)
	user.StartedAt = pkg.NullStringToString(started_at)
	user.FinishedAt = pkg.NullStringToString(finished_at)
	user.CreatedAt = pkg.NullStringToString(created_at)
	user.UpdatedAt = pkg.NullStringToString(updated_at)

	return &user, nil
}

func (c *userRepo) GetPassword(ctx context.Context, login string) (string, error) {
	var hashedPass string

	query := `SELECT user_password
	FROM users
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query, login).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("incorrect login")
		} else {
			log.Println("failed to get user password from database", logger.Error(err))
			return "", err
		}
	}

	return hashedPass, nil
}
