package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	sa "go_user_service/genproto/superadmin_service"
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

type superadminRepo struct {
	db *pgxpool.Pool
}

func NewSuperadminRepo(db *pgxpool.Pool) storage.SuperadminRepoI {
	return &superadminRepo{
		db: db,
	}
}

func generateSuperadminLogin(db *pgxpool.Pool, ctx context.Context) (string, error) {
	var nextVal int
	err := db.QueryRow(ctx, "SELECT nextval('superadmin_external_id_seq')").Scan(&nextVal)
	if err != nil {
		return "", err
	}
	userLogin := "SA" + fmt.Sprintf("%05d", nextVal)
	return userLogin, nil
}

func (c *superadminRepo) Create(ctx context.Context, req *sa.CreateSuperadmin) (*sa.GetSuperadmin, error) {

	id := uuid.NewString()
	pasword, err := hash.HashPassword(req.UserPassword)
	if err != nil {
		log.Println("error while hashing password", err)
	}

	userLogin, err := generateSuperadminLogin(c.db, ctx)
	if err != nil {
		log.Println("error while generating login", err)
	}
	comtag, err := c.db.Exec(ctx, `
		INSERT INTO superadmins (
			id,
			user_login,
			birthday,
			gender,
			fullname,
			email,
			phone,
			user_password,
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8
		)`,
		id,
		userLogin,
		req.Birthday,
		req.Gender,
		req.Fullname,
		req.Email,
		req.Phone,
		pasword)
	if err != nil {
		log.Println("error while creating superadmin", comtag)
		return nil, err
	}

	superadmin, err := c.GetById(ctx, &sa.SuperadminPrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting superadmin by id")
		return nil, err
	}
	return superadmin, nil
}

func (c *superadminRepo) Update(ctx context.Context, req *sa.UpdateSuperadmin) (*sa.GetSuperadmin, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE superadmins SET
		birthday = $1,
		gender = $2,
		fullname = $3,
		email = $4,
		phone = $5,
		updated_at = NOW()
		WHERE id = $6
		`,
		req.Birthday,
		req.Gender,
		req.Fullname,
		req.Email,
		req.Phone,
		req.Id)
	if err != nil {
		log.Println("error while updating superadmin")
		return nil, err
	}

	superadmin, err := c.GetById(ctx, &sa.SuperadminPrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting superadmin by id")
		return nil, err
	}
	return superadmin, nil
}

func (c *superadminRepo) GetById(ctx context.Context, id *sa.SuperadminPrimaryKey) (*sa.GetSuperadmin, error) {
	var (
		superadmin sa.GetSuperadmin
		created_at sql.NullString
		updated_at sql.NullString
	)

	query := `SELECT
				id,
				birthday,
				gender,
				fullname,
				email,
				phone,
				created_at,
				updated_at
			FROM superadmins
			WHERE id = $1 AND deleted_at IS NULL`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&superadmin.Id,
		&superadmin.Birthday,
		&superadmin.Gender,
		&superadmin.Fullname,
		&superadmin.Email,
		&superadmin.Phone,
		&created_at,
		&updated_at); err != nil {
		return &superadmin, err
	}

	superadmin.CreatedAt = pkg.NullStringToString(created_at)
	superadmin.UpdatedAt = pkg.NullStringToString(updated_at)

	return &superadmin, nil
}

func (c *superadminRepo) Delete(ctx context.Context, id *sa.SuperadminPrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE superadmins SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}

//////////////////////////////////////////////////////

func (c *superadminRepo) ChangePassword(ctx context.Context, pass *sa.SuperadminChangePassword) (*sa.SuperadminChangePasswordResp, error) {
	var hashedPass string
	var resp sa.SuperadminChangePasswordResp
	query := `SELECT user_password
	FROM superadmins
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query,
		pass.UserLogin,
	).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("incorrect login")
		}
		log.Println("failed to get superadmin password from database", logger.Error(err))
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(pass.OldPassword))
	if err != nil {
		return nil, errors.New("password mismatch")
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(pass.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Println("failed to generate superadmin new password", logger.Error(err))
		return nil, err
	}

	query = `UPDATE superadmins SET 
		user_password = $1, 
		updated_at = NOW() 
	WHERE user_login = $2 AND deleted_at is null`

	_, err = c.db.Exec(ctx, query, newHashedPassword, pass.UserLogin)
	if err != nil {
		log.Println("failed to change superadmin password in database", logger.Error(err))
		return nil, err
	}
	resp.Comment = "Password changed successfully"

	return &resp, nil
}

func (c *superadminRepo) GetByLogin(ctx context.Context, login string) (*sa.GetSuperadminByLogin, error) {
	var (
		superadmin sa.GetSuperadminByLogin
		birthday   sql.NullString
		created_at sql.NullString
		updated_at sql.NullString
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
		created_at, 
		updated_at
		FROM superadmins WHERE user_login = $1 AND deleted_at is null`

	row := c.db.QueryRow(ctx, query, login)

	err := row.Scan(
		&superadmin.Id,
		&superadmin.BranchId,
		&superadmin.UserLogin,
		&birthday,
		&superadmin.Gender,
		&superadmin.Fullname,
		&superadmin.Email,
		&superadmin.Phone,
		&superadmin.UserPassword,
		&created_at,
		&updated_at,
	)

	if err != nil {
		log.Println("failed to scan superadmin by LOGIN from database", logger.Error(err))
		return &sa.GetSuperadminByLogin{}, err
	}

	superadmin.Birthday = pkg.NullStringToString(birthday)
	superadmin.CreatedAt = pkg.NullStringToString(created_at)
	superadmin.UpdatedAt = pkg.NullStringToString(updated_at)

	return &superadmin, nil
}

func (c *superadminRepo) GetPassword(ctx context.Context, login string) (string, error) {
	var hashedPass string

	query := `SELECT user_password
	FROM superadmins
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query, login).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("incorrect login")
		} else {
			log.Println("failed to get superadmin password from database", logger.Error(err))
			return "", err
		}
	}

	return hashedPass, nil
}
