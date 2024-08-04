package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	adm "go_user_service/genproto/admin_service"
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

type adminRepo struct {
	db *pgxpool.Pool
}

func NewAdminRepo(db *pgxpool.Pool) storage.AdminRepoI {
	return &adminRepo{
		db: db,
	}
}

func generateAdminLogin(db *pgxpool.Pool, ctx context.Context) (string, error) {
	var nextVal int
	err := db.QueryRow(ctx, "SELECT nextval('admin_external_id_seq')").Scan(&nextVal)
	if err != nil {
		return "", err
	}
	userLogin := "A" + fmt.Sprintf("%05d", nextVal)
	return userLogin, nil
}

func (c *adminRepo) Create(ctx context.Context, req *adm.CreateAdmin) (*adm.GetAdmin, error) {
	id := uuid.NewString()
	pasword, err := hash.HashPassword(req.UserPassword)
	if err != nil {
		log.Println("error while hashing password", err)
	}

	userLogin, err := generateAdminLogin(c.db, ctx)
	if err != nil {
		log.Println("error while generating login", err)
	}
	comtag, err := c.db.Exec(ctx, `
		INSERT INTO admins (
			id,
			user_login,
			birthday,
			gender,
			fullname,
			email,
			phone,
			user_password
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
		log.Println("error while creating admin", comtag)
		return nil, err
	}

	admin, err := c.GetById(ctx, &adm.AdminPrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting admin by id")
		return nil, err
	}
	return admin, nil
}

func (c *adminRepo) Update(ctx context.Context, req *adm.UpdateAdmin) (*adm.GetAdmin, error) {
	_, err := c.db.Exec(ctx, `
		UPDATE admins SET
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
		log.Println("error while updating admin")
		return nil, err
	}

	admin, err := c.GetById(ctx, &adm.AdminPrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting admin by id")
		return nil, err
	}
	return admin, nil
}

func (c *adminRepo) GetAll(ctx context.Context, req *adm.GetListAdminRequest) (*adm.GetListAdminResponse, error) {
	admins := adm.GetListAdminResponse{}
	var (
		created_at sql.NullString
		updated_at sql.NullString
		birthday   sql.NullString
	)
	filter_by_name := ""
	offest := (req.Offset - 1) * req.Limit
	if req.Search != "" {
		filter_by_name = fmt.Sprintf(`AND fullname ILIKE '%%%v%%'`, req.Search)
	}
	query := `SELECT
				id,
				user_login,
				birthday,
				gender,
				fullname,
				email,
				phone,
				created_at,
				updated_at
			FROM admins
			WHERE TRUE AND deleted_at is null ` + filter_by_name + `
			OFFSET $1 LIMIT $2
`
	rows, err := c.db.Query(ctx, query, offest, req.Limit)

	if err != nil {
		log.Println("error while getting all admins")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			admin adm.GetAdmin
		)
		if err = rows.Scan(
			&admin.Id,
			&admin.UserLogin,
			&birthday,
			&admin.Gender,
			&admin.Fullname,
			&admin.Email,
			&admin.Phone,
			&created_at,
			&updated_at,
		); err != nil {
			return &admins, err
		}
		admin.Birthday = pkg.NullStringToString(birthday)
		admin.CreatedAt = pkg.NullStringToString(created_at)
		admin.UpdatedAt = pkg.NullStringToString(updated_at)

		admins.Admins = append(admins.Admins, &admin)
	}

	err = c.db.QueryRow(ctx, `SELECT count(*) from admins WHERE TRUE AND deleted_at is null `+filter_by_name+``).Scan(&admins.Count)
	if err != nil {
		return &admins, err
	}

	return &admins, nil
}

func (c *adminRepo) GetById(ctx context.Context, id *adm.AdminPrimaryKey) (*adm.GetAdmin, error) {
	var (
		admin      adm.GetAdmin
		birthday   sql.NullString
		created_at sql.NullString
		updated_at sql.NullString
	)

	query := `SELECT
				id,
				user_login,
				birthday,
				gender,
				fullname,
				email,
				phone,
				created_at,
				updated_at
			FROM admins
			WHERE id = $1 AND deleted_at IS NULL`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&admin.Id,
		&admin.UserLogin,
		&birthday,
		&admin.Gender,
		&admin.Fullname,
		&admin.Email,
		&admin.Phone,
		&created_at,
		&updated_at); err != nil {
		return &admin, err
	}
	admin.Birthday = pkg.NullStringToString(birthday)
	admin.CreatedAt = pkg.NullStringToString(created_at)
	admin.UpdatedAt = pkg.NullStringToString(updated_at)

	return &admin, nil
}

func (c *adminRepo) Delete(ctx context.Context, id *adm.AdminPrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE admins SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}

///////////////////////////////////////////

func (c *adminRepo) ChangePassword(ctx context.Context, pass *adm.AdminChangePassword) (*adm.AdminChangePasswordResp, error) {
	var hashedPass string
	var resp adm.AdminChangePasswordResp
	query := `SELECT user_password
	FROM admins
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query,
		pass.UserLogin,
	).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("incorrect login")
		}
		log.Println("failed to get admin password from database", logger.Error(err))
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(pass.OldPassword))
	if err != nil {
		return nil, errors.New("password mismatch")
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(pass.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Println("failed to generate admin new password", logger.Error(err))
		return nil, err
	}

	query = `UPDATE admins SET 
		user_password = $1, 
		updated_at = NOW() 
	WHERE user_login = $2 AND deleted_at is null`

	_, err = c.db.Exec(ctx, query, newHashedPassword, pass.UserLogin)
	if err != nil {
		log.Println("failed to change admin password in database", logger.Error(err))
		return nil, err
	}
	resp.Comment = "Password changed successfully"
	return &resp, nil
}

func (c *adminRepo) GetByLogin(ctx context.Context, login string) (*adm.GetAdminByLogin, error) {
	var (
		admin      adm.GetAdminByLogin
		birthday   sql.NullString
		created_at sql.NullString
		updated_at sql.NullString
	)

	query := `SELECT 
		id, 
		user_login,
		birthday, 
		gender,
		fullname,
		email,
		phone,
		user_password,
		created_at, 
		updated_at
		FROM admins WHERE user_login = $1 AND deleted_at is null`

	row := c.db.QueryRow(ctx, query, login)

	err := row.Scan(
		&admin.Id,
		&admin.UserLogin,
		&birthday,
		&admin.Gender,
		&admin.Fullname,
		&admin.Email,
		&admin.Phone,
		&admin.UserPassword,
		&created_at,
		&updated_at,
	)

	if err != nil {
		log.Println("failed to scan admin by LOGIN from database", logger.Error(err))
		return &adm.GetAdminByLogin{}, err
	}

	admin.Birthday = pkg.NullStringToString(birthday)
	admin.CreatedAt = pkg.NullStringToString(created_at)
	admin.UpdatedAt = pkg.NullStringToString(updated_at)

	return &admin, nil
}

func (c *adminRepo) GetPassword(ctx context.Context, login string) (string, error) {
	var hashedPass string

	query := `SELECT user_password
	FROM admins
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query, login).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("incorrect login")
		} else {
			log.Println("failed to get admin password from database", logger.Error(err))
			return "", err
		}
	}

	return hashedPass, nil
}
