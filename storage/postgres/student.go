package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	stud "go_user_service/genproto/student_service"
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

type studentRepo struct {
	db *pgxpool.Pool
}

func NewStudentRepo(db *pgxpool.Pool) storage.StudentRepoI {
	return &studentRepo{
		db: db,
	}
}

func generateStudentLogin(db *pgxpool.Pool, ctx context.Context) (string, error) {
	var nextVal int
	err := db.QueryRow(ctx, "SELECT nextval('student_external_id_seq')").Scan(&nextVal)
	if err != nil {
		return "", err
	}
	userLogin := "S" + fmt.Sprintf("%05d", nextVal)
	return userLogin, nil
}

func (c *studentRepo) Create(ctx context.Context, req *stud.CreateStudent) (*stud.GetStudent, error) {
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

	userLogin, err := generateStudentLogin(c.db, ctx)
	if err != nil {
		log.Println("error while generating login", err)
	}

	comtag, err := c.db.Exec(ctx, `
		INSERT INTO students (
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
		log.Println("error while creating student", comtag)
		return nil, err
	}

	student, err := c.GetById(ctx, &stud.StudentPrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting student by id")
		return nil, err
	}
	return student, nil
}

func (c *studentRepo) Update(ctx context.Context, req *stud.UpdateStudent) (*stud.GetStudent, error) {
	var finished_at sql.NullString
	if req.FinishedAt == "" {
		finished_at = sql.NullString{Valid: false}
	} else {
		finished_at = sql.NullString{String: req.FinishedAt, Valid: true}
	}
	_, err := c.db.Exec(ctx, `
		UPDATE students SET
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
		log.Println("error while updating student")
		return nil, err
	}

	student, err := c.GetById(ctx, &stud.StudentPrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting student by id")
		return nil, err
	}
	return student, nil
}

func (c *studentRepo) GetAll(ctx context.Context, req *stud.GetListStudentRequest) (*stud.GetListStudentResponse, error) {
	students := stud.GetListStudentResponse{}

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
			FROM students
			WHERE TRUE AND deleted_at is null ` + filter_by_name + `
			OFFSET $1 LIMIT $2
`
	rows, err := c.db.Query(ctx, query, offest, req.Limit)

	if err != nil {
		log.Println("error while getting all students")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			student stud.GetStudent
		)
		if err = rows.Scan(
			&student.Id,
			&student.GroupId,
			&student.UserLogin,
			&student.Birthday,
			&student.Gender,
			&student.Fullname,
			&student.Email,
			&student.Phone,
			&student.PaidSum,
			&started_at,
			&finished_at,
			&created_at,
			&updated_at,
		); err != nil {
			return &students, err
		}
		student.StartedAt = pkg.NullStringToString(started_at)
		student.FinishedAt = pkg.NullStringToString(finished_at)
		student.CreatedAt = pkg.NullStringToString(created_at)
		student.UpdatedAt = pkg.NullStringToString(updated_at)

		students.Students = append(students.Students, &student)
	}

	err = c.db.QueryRow(ctx, `SELECT count(*) from students WHERE TRUE AND deleted_at is null `+filter_by_name+``).Scan(&students.Count)
	if err != nil {
		return &students, err
	}

	return &students, nil
}

func (c *studentRepo) GetById(ctx context.Context, id *stud.StudentPrimaryKey) (*stud.GetStudent, error) {
	var (
		student     stud.GetStudent
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
			FROM students
			WHERE id = $1 AND deleted_at IS NULL`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&student.Id,
		&student.GroupId,
		&student.UserLogin,
		&student.Birthday,
		&student.Gender,
		&student.Fullname,
		&student.Email,
		&student.Phone,
		&student.PaidSum,
		&started_at,
		&finished_at,
		&created_at,
		&updated_at); err != nil {
		return &student, err
	}
	student.StartedAt = pkg.NullStringToString(started_at)
	student.FinishedAt = pkg.NullStringToString(finished_at)
	student.CreatedAt = pkg.NullStringToString(created_at)
	student.UpdatedAt = pkg.NullStringToString(updated_at)

	return &student, nil
}

func (c *studentRepo) Delete(ctx context.Context, id *stud.StudentPrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE students SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}

func (c *studentRepo) Check(ctx context.Context, id *stud.StudentPrimaryKey) (*stud.CheckStudentResp, error) {
	query := `SELECT EXISTS (
                SELECT 1
                FROM students
                WHERE id = $1 AND deleted_at IS NULL
            )`

	var exists bool
	err := c.db.QueryRow(ctx, query, id.Id).Scan(&exists)
	if err != nil {
		return nil, err
	}

	resp := &stud.CheckStudentResp{
		Check: exists,
	}

	return resp, nil
}

func (c *studentRepo) ChangePassword(ctx context.Context, pass *stud.StudentChangePassword) (*stud.StudentChangePasswordResp, error) {
	var hashedPass string
	var resp stud.StudentChangePasswordResp
	query := `SELECT user_password
	FROM students
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query,
		pass.UserLogin,
	).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("incorrect login")
		}
		log.Println("failed to get student password from database", logger.Error(err))
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPass), []byte(pass.OldPassword))
	if err != nil {
		return nil, errors.New("password mismatch")
	}

	newHashedPassword, err := bcrypt.GenerateFromPassword([]byte(pass.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Println("failed to generate student new password", logger.Error(err))
		return nil, err
	}

	query = `UPDATE students SET 
		user_password = $1, 
		updated_at = NOW() 
	WHERE user_login = $2 AND deleted_at is null`

	_, err = c.db.Exec(ctx, query, newHashedPassword, pass.UserLogin)
	if err != nil {
		log.Println("failed to change student password in database", logger.Error(err))
		return nil, err
	}
	resp.Comment = "Password changed successfully"

	return &resp, nil
}

func (c *studentRepo) GetByLogin(ctx context.Context, login string) (*stud.GetStudentByLogin, error) {
	var (
		student     stud.GetStudentByLogin
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
		FROM students WHERE user_login = $1 AND deleted_at is null`

	row := c.db.QueryRow(ctx, query, login)

	err := row.Scan(
		&student.Id,
		&student.GroupId,
		&student.UserLogin,
		&birthday,
		&student.Gender,
		&student.Fullname,
		&student.Email,
		&student.Phone,
		&student.UserPassword,
		&student.PaidSum,
		&started_at,
		&finished_at,
		&created_at,
		&updated_at,
	)

	if err != nil {
		log.Println("failed to scan student by LOGIN from database", logger.Error(err))
		return &stud.GetStudentByLogin{}, err
	}

	student.Birthday = pkg.NullStringToString(birthday)
	student.StartedAt = pkg.NullStringToString(started_at)
	student.FinishedAt = pkg.NullStringToString(finished_at)
	student.CreatedAt = pkg.NullStringToString(created_at)
	student.UpdatedAt = pkg.NullStringToString(updated_at)

	return &student, nil
}

func (c *studentRepo) GetPassword(ctx context.Context, login string) (string, error) {
	var hashedPass string

	query := `SELECT user_password
	FROM students
	WHERE user_login = $1 AND deleted_at is null`

	err := c.db.QueryRow(ctx, query, login).Scan(&hashedPass)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", errors.New("incorrect login")
		} else {
			log.Println("failed to get student password from database", logger.Error(err))
			return "", err
		}
	}

	return hashedPass, nil
}
