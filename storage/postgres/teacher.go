package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go_user_service/genproto/teacher_service"
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

type teacherRepo struct {
	db *pgxpool.Pool
}

func NewTeacherRepo(db *pgxpool.Pool) storage.TeacherRepoI {
	return &teacherRepo{
		db: db,
	}
}

func generateTeacherLogin(db *pgxpool.Pool, ctx context.Context) (string, error) {
	var nextVal int
	err := db.QueryRow(ctx, "SELECT nextval('teacher_external_id_seq')").Scan(&nextVal)
	if err != nil {
		return "", err
	}
	userLogin := "T" + fmt.Sprintf("%05d", nextVal)
	return userLogin, nil
}

func (c *teacherRepo) Create(ctx context.Context, req *teacher_service.CreateTeacher) (*teacher_service.GetTeacher, error) {
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

	userLogin, err := generateTeacherLogin(c.db, ctx)
	if err != nil {
		log.Println("error while generating login", err)
	}
	comtag, err := c.db.Exec(ctx, `
		INSERT INTO teachers (
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
			ielts_score,
			ielts_attempts_count,
			start_working,
			end_working
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14
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
		req.IeltsScore,
		req.IeltsAttemptsCount,
		req.StartWorking,
		end_working)
	if err != nil {
		log.Println("error while creating teacher", comtag)
		return nil, err
	}

	teacher, err := c.GetById(ctx, &teacher_service.TeacherPrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting teacher by id")
		return nil, err
	}
	return teacher, nil
}

func (c *teacherRepo) Update(ctx context.Context, req *teacher_service.UpdateTeacher) (*teacher_service.GetTeacher, error) {
	var end_working sql.NullString
	if req.EndWorking == "" {
		end_working = sql.NullString{Valid: false}
	} else {
		end_working = sql.NullString{String: req.EndWorking, Valid: true}
	}
	_, err := c.db.Exec(ctx, `
		UPDATE teachers SET
		branch_id = $1,
		birthday = $2,
		gender = $3,
		fullname = $4,
		email = $5,
		phone = $6,
		salary = $7,
		ielts_score = $8,
		ielts_attempts_count = $9,
		start_working = $10,
		end_working = $11,
		updated_at = NOW()
		WHERE id = $12
		`,
		req.BranchId,
		req.Birthday,
		req.Gender,
		req.Fullname,
		req.Email,
		req.Phone,
		req.Salary,
		req.IeltsScore,
		req.IeltsAttemptsCount,
		req.StartWorking,
		end_working,
		req.Id)
	if err != nil {
		log.Println("error while updating teacher")
		return nil, err
	}

	teacher, err := c.GetById(ctx, &teacher_service.TeacherPrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting teacher by id")
		return nil, err
	}
	return teacher, nil
}

func (c *teacherRepo) GetAll(ctx context.Context, req *teacher_service.GetListTeacherRequest) (*teacher_service.GetListTeacherResponse, error) {
	teachers := teacher_service.GetListTeacherResponse{}
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
				ielts_score,
				ielts_attempts_count,
				start_working,
				end_working,
				created_at,
				updated_at
			FROM teachers
			WHERE TRUE AND deleted_at is null ` + filter_by_name + `
			OFFSET $1 LIMIT $2
`
	rows, err := c.db.Query(ctx, query, offest, req.Limit)

	if err != nil {
		log.Println("error while getting all teachers")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			teacher teacher_service.GetTeacher
		)
		if err = rows.Scan(
			&teacher.Id,
			&teacher.BranchId,
			&teacher.UserLogin,
			&teacher.Birthday,
			&teacher.Gender,
			&teacher.Fullname,
			&teacher.Email,
			&teacher.Phone,
			&teacher.Salary,
			&teacher.IeltsScore,
			&teacher.IeltsAttemptsCount,
			&start_working,
			&end_working,
			&created_at,
			&updated_at,
		); err != nil {
			return &teachers, err
		}
		teacher.StartWorking = pkg.NullStringToString(start_working)
		teacher.EndWorking = pkg.NullStringToString(end_working)
		teacher.CreatedAt = pkg.NullStringToString(created_at)
		teacher.UpdatedAt = pkg.NullStringToString(updated_at)

		teachers.Teachers = append(teachers.Teachers, &teacher)
	}

	err = c.db.QueryRow(ctx, `SELECT count(*) from teachers WHERE TRUE AND deleted_at is null `+filter_by_name+``).Scan(&teachers.Count)
	if err != nil {
		return &teachers, err
	}

	return &teachers, nil
}

func (c *teacherRepo) GetById(ctx context.Context, id *teacher_service.TeacherPrimaryKey) (*teacher_service.GetTeacher, error) {
	var (
		teacher       teacher_service.GetTeacher
		created_at    sql.NullString
		updated_at    sql.NullString
		start_working sql.NullString
		end_working   sql.NullString
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
				salary,
				ielts_score,
				ielts_attempts_count,
				start_working,
				end_working,
				created_at,
				updated_at
			FROM teachers
			WHERE id = $1 AND deleted_at IS NULL`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&teacher.Id,
		&teacher.BranchId,
		&teacher.UserLogin,
		&teacher.Birthday,
		&teacher.Gender,
		&teacher.Fullname,
		&teacher.Email,
		&teacher.Phone,
		&teacher.Salary,
		&teacher.IeltsScore,
		&teacher.IeltsAttemptsCount,
		&start_working,
		&end_working,
		&created_at,
		&updated_at); err != nil {
		return &teacher, err
	}
	teacher.StartWorking = pkg.NullStringToString(start_working)
	teacher.EndWorking = pkg.NullStringToString(end_working)
	teacher.CreatedAt = pkg.NullStringToString(created_at)
	teacher.UpdatedAt = pkg.NullStringToString(updated_at)

	return &teacher, nil
}

func (c *teacherRepo) Delete(ctx context.Context, id *teacher_service.TeacherPrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE teachers SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}

///////////////////////////////////////////////////1222222222222222222222222222222222222222222222222222222222222222

/////////////////////////////////////////////////////33333333333333333333333333333333333333333333333333333333333333

func (c *teacherRepo) ChangePassword(ctx context.Context, pass *teacher_service.TeacherChangePassword) (*teacher_service.TeacherChangePasswordResp, error) {
	var hashedPass string
	var resp teacher_service.TeacherChangePasswordResp

	query := `SELECT user_password
	FROM teachers
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query,
		pass.UserLogin,
	).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("incorrect login")
		}
		log.Println("failed to get teacher password from database", logger.Error(err))
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(pass.OldPassword))
	if err != nil {
		return nil, errors.New("password mismatch")
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(pass.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Println("failed to generate teacher new password", logger.Error(err))
		return nil, err
	}

	query = `UPDATE teachers SET 
		user_password = $1, 
		updated_at = NOW() 
	WHERE user_login = $2 AND deleted_at is null`

	_, err = c.db.Exec(ctx, query, newHashedPassword, pass.UserLogin)
	if err != nil {
		log.Println("failed to change teacher password in database", logger.Error(err))
		return &resp, err
	}
	resp.Comment = "Password changed successfully"
	return &resp, nil
}

func (c *teacherRepo) GetByLogin(ctx context.Context, login string) (*teacher_service.GetTeacherByLogin, error) {
	var (
		teacher       teacher_service.GetTeacherByLogin
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
		ielts_score,
		ielts_attempts_count,
		start_working,
		end_working,
		created_at, 
		updated_at
		FROM teachers WHERE user_login = $1 AND deleted_at is null`

	row := c.db.QueryRow(ctx, query, login)

	err := row.Scan(
		&teacher.Id,
		&teacher.BranchId,
		&teacher.UserLogin,
		&birthday,
		&teacher.Gender,
		&teacher.Fullname,
		&teacher.Email,
		&teacher.Phone,
		&teacher.UserPassword,
		&teacher.Salary,
		&teacher.IeltsScore,
		&teacher.IeltsAttemptsCount,
		&start_working,
		&end_working,
		&created_at,
		&updated_at,
	)

	if err != nil {
		log.Println("failed to scan teacher by LOGIN from database", logger.Error(err))
		return &teacher_service.GetTeacherByLogin{}, err
	}

	teacher.Birthday = pkg.NullStringToString(birthday)
	teacher.StartWorking = pkg.NullStringToString(start_working)
	teacher.EndWorking = pkg.NullStringToString(end_working)
	teacher.CreatedAt = pkg.NullStringToString(created_at)
	teacher.UpdatedAt = pkg.NullStringToString(updated_at)

	return &teacher, nil
}

func (c *teacherRepo) GetPassword(ctx context.Context, login string) (string, error) {
	var hashedPass string

	query := `SELECT user_password
	FROM teachers
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query, login).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("incorrect login")
		} else {
			log.Println("failed to get teacher password from database", logger.Error(err))
			return "", err
		}
	}

	return hashedPass, nil
}
