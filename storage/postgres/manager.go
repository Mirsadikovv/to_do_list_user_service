package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	man "go_user_service/genproto/manager_service"
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

type managerRepo struct {
	db *pgxpool.Pool
}

func NewManagerRepo(db *pgxpool.Pool) storage.ManagerRepoI {
	return &managerRepo{
		db: db,
	}
}

func generateManagerLogin(db *pgxpool.Pool, ctx context.Context) (string, error) {
	var nextVal int
	err := db.QueryRow(ctx, "SELECT nextval('manager_external_id_seq')").Scan(&nextVal)
	if err != nil {
		return "", err
	}
	userLogin := "M" + fmt.Sprintf("%05d", nextVal)
	return userLogin, nil
}

func (c *managerRepo) Create(ctx context.Context, req *man.CreateManager) (*man.GetManager, error) {
	var end_working sql.NullString
	if req.EndWorking == "" {
		end_working = sql.NullString{Valid: false}
	} else {
		end_working = sql.NullString{String: req.EndWorking, Valid: true}
	}
	id := uuid.NewString()
	pasword, err := hash.HashPassword(req.UserPassword)
	if err != nil {
		log.Println("error while hashing password", err)
	}

	userLogin, err := generateManagerLogin(c.db, ctx)
	if err != nil {
		log.Println("error while generating login", err)
	}
	comtag, err := c.db.Exec(ctx, `
		INSERT INTO managers (
			id,
			branch_id,
			user_login,
			birthday,
			gender,
			fullname,
			email,
			phone,
			user_password,
			salary,
			start_working,
			end_working
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12
		)`,
		id,
		req.BranchId,
		userLogin,
		req.Birthday,
		req.Gender,
		req.Fullname,
		req.Email,
		req.Phone,
		pasword,
		req.Salary,
		req.StartWorking,
		end_working)
	if err != nil {
		log.Println("error while creating manager", comtag)
		return nil, err
	}

	manager, err := c.GetById(ctx, &man.ManagerPrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting manager by id")
		return nil, err
	}
	return manager, nil
}

func (c *managerRepo) Update(ctx context.Context, req *man.UpdateManager) (*man.GetManager, error) {
	var end_working sql.NullString
	if req.EndWorking == "" {
		end_working = sql.NullString{Valid: false}
	} else {
		end_working = sql.NullString{String: req.EndWorking, Valid: true}
	}
	_, err := c.db.Exec(ctx, `
		UPDATE managers SET
		branch_id = $1,
		birthday = $2,
		gender = $3,
		fullname = $4,
		email = $5,
		phone = $6,
		salary = $7,
		start_working = $8,
		end_working = $9,
		updated_at = NOW()
		WHERE id = $10
		`,
		req.BranchId,
		req.Birthday,
		req.Gender,
		req.Fullname,
		req.Email,
		req.Phone,
		req.Salary,
		req.StartWorking,
		end_working,
		req.Id)
	if err != nil {
		log.Println("error while updating manager")
		return nil, err
	}

	manager, err := c.GetById(ctx, &man.ManagerPrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting manager by id")
		return nil, err
	}
	return manager, nil
}

func (c *managerRepo) GetAll(ctx context.Context, req *man.GetListManagerRequest) (*man.GetListManagerResponse, error) {
	managers := man.GetListManagerResponse{}
	var (
		created_at    sql.NullString
		updated_at    sql.NullString
		start_working sql.NullString
		end_working   sql.NullString
	)
	filter_by_name := ""
	offest := (req.Offset - 1) * req.Limit
	if req.Search != "" {
		filter_by_name = fmt.Sprintf(`AND fullname ILIKE '%%%v%%'`, req.Search)
	}
	query := `SELECT
				id,
				branch_id,
				user_login,
				birthday,
				gender,
				fullname,
				email,
				phone,
				salary,
				start_working,
				end_working,
				created_at,
				updated_at
			FROM managers
			WHERE TRUE AND deleted_at is null ` + filter_by_name + `
			OFFSET $1 LIMIT $2
`
	rows, err := c.db.Query(ctx, query, offest, req.Limit)

	if err != nil {
		log.Println("error while getting all managers")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			manager man.GetManager
		)
		if err = rows.Scan(
			&manager.Id,
			&manager.BranchId,
			&manager.UserLogin,
			&manager.Birthday,
			&manager.Gender,
			&manager.Fullname,
			&manager.Email,
			&manager.Phone,
			&manager.Salary,
			&start_working,
			&end_working,
			&created_at,
			&updated_at,
		); err != nil {
			return &managers, err
		}
		manager.StartWorking = pkg.NullStringToString(start_working)
		manager.EndWorking = pkg.NullStringToString(end_working)
		manager.CreatedAt = pkg.NullStringToString(created_at)
		manager.UpdatedAt = pkg.NullStringToString(updated_at)

		managers.Managers = append(managers.Managers, &manager)
	}

	err = c.db.QueryRow(ctx, `SELECT count(*) from managers WHERE TRUE AND deleted_at is null `+filter_by_name+``).Scan(&managers.Count)
	if err != nil {
		return &managers, err
	}

	return &managers, nil
}

func (c *managerRepo) GetById(ctx context.Context, id *man.ManagerPrimaryKey) (*man.GetManager, error) {
	var (
		manager       man.GetManager
		created_at    sql.NullString
		updated_at    sql.NullString
		start_working sql.NullString
		end_working   sql.NullString
	)

	query := `SELECT
				id,
				branch_id,
				birthday,
				gender,
				fullname,
				email,
				phone,
				salary,
				start_working,
				end_working,
				created_at,
				updated_at
			FROM managers
			WHERE id = $1 AND deleted_at IS NULL`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&manager.Id,
		&manager.BranchId,
		&manager.Birthday,
		&manager.Gender,
		&manager.Fullname,
		&manager.Email,
		&manager.Phone,
		&manager.Salary,
		&start_working,
		&end_working,
		&created_at,
		&updated_at); err != nil {
		return &manager, err
	}
	manager.StartWorking = pkg.NullStringToString(start_working)
	manager.EndWorking = pkg.NullStringToString(end_working)
	manager.CreatedAt = pkg.NullStringToString(created_at)
	manager.UpdatedAt = pkg.NullStringToString(updated_at)

	return &manager, nil
}

func (c *managerRepo) Delete(ctx context.Context, id *man.ManagerPrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE managers SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}

/////////////////////////////////////////////

func (c *managerRepo) ChangePassword(ctx context.Context, pass *man.ManagerChangePassword) (*man.ManagerChangePasswordResp, error) {
	var hashedPass string
	var resp man.ManagerChangePasswordResp
	query := `SELECT user_password
	FROM managers
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query,
		pass.UserLogin,
	).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("incorrect login")
		}
		log.Println("failed to get manager password from database", logger.Error(err))
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(pass.OldPassword))
	if err != nil {
		return nil, errors.New("password mismatch")
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(pass.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Println("failed to generate manager new password", logger.Error(err))
		return nil, err
	}

	query = `UPDATE managers SET 
		user_password = $1, 
		updated_at = NOW() 
	WHERE user_login = $2 AND deleted_at is null`

	_, err = c.db.Exec(ctx, query, newHashedPassword, pass.UserLogin)
	if err != nil {
		log.Println("failed to change manager password in database", logger.Error(err))
		return nil, err
	}
	resp.Comment = "Password changed successfully"
	return &resp, nil
}

func (c *managerRepo) GetByLogin(ctx context.Context, login string) (*man.GetManagerByLogin, error) {
	var (
		manager       man.GetManagerByLogin
		birthday      sql.NullString
		start_working sql.NullString
		end_working   sql.NullString
		created_at    sql.NullString
		updated_at    sql.NullString
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
		salary,
		start_working,
		end_working,
		created_at, 
		updated_at
		FROM managers WHERE user_login = $1 AND deleted_at is null`

	row := c.db.QueryRow(ctx, query, login)

	err := row.Scan(
		&manager.Id,
		&manager.BranchId,
		&manager.UserLogin,
		&birthday,
		&manager.Gender,
		&manager.Fullname,
		&manager.Email,
		&manager.Phone,
		&manager.UserPassword,
		&manager.Salary,
		&start_working,
		&end_working,
		&created_at,
		&updated_at,
	)

	if err != nil {
		log.Println("failed to scan manager by LOGIN from database", logger.Error(err))
		return &man.GetManagerByLogin{}, err
	}

	manager.Birthday = pkg.NullStringToString(birthday)
	manager.StartWorking = pkg.NullStringToString(start_working)
	manager.EndWorking = pkg.NullStringToString(end_working)
	manager.CreatedAt = pkg.NullStringToString(created_at)
	manager.UpdatedAt = pkg.NullStringToString(updated_at)

	return &manager, nil
}

func (c *managerRepo) GetPassword(ctx context.Context, login string) (string, error) {
	var hashedPass string

	query := `SELECT user_password
	FROM managers
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query, login).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("incorrect login")
		} else {
			log.Println("failed to get manager password from database", logger.Error(err))
			return "", err
		}
	}

	return hashedPass, nil
}
