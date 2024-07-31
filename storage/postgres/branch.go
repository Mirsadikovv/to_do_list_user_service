package postgres

import (
	"context"
	"database/sql"
	"fmt"
	br "go_user_service/genproto/branch_service"
	"go_user_service/pkg"
	"go_user_service/storage"
	"log"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

type branchRepo struct {
	db *pgxpool.Pool
}

func NewBranchRepo(db *pgxpool.Pool) storage.BranchRepoI {
	return &branchRepo{
		db: db,
	}
}

func (c *branchRepo) Create(ctx context.Context, req *br.CreateBranch) (*br.GetBranch, error) {
	id := uuid.NewString()

	comtag, err := c.db.Exec(ctx, `
		INSERT INTO branches (
			id,
			branch_name,
			branch_location,
			phone,
			open_time,
			close_time
		) VALUES ($1,$2,$3,$4,$5,$6
		)`,
		id,
		req.BranchName,
		req.BranchLocation,
		req.Phone,
		req.OpenTime,
		req.CloseTime,
	)
	if err != nil {
		log.Println("error while creating branch", comtag)
		return nil, err
	}

	branch, err := c.GetById(ctx, &br.BranchPrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting branch by id")
		return nil, err
	}
	return branch, nil
}

func (c *branchRepo) Update(ctx context.Context, req *br.UpdateBranch) (*br.GetBranch, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE branches SET
		branch_name = $1,
		branch_location = $2,
		phone = $3,
		open_time = $4,
		close_time = $5,
		updated_at = NOW()
		WHERE id = $6
		`,
		req.BranchName,
		req.BranchLocation,
		req.Phone,
		req.OpenTime,
		req.CloseTime,
		req.Id,
	)
	if err != nil {
		log.Println("error while updating branch")
		return nil, err
	}

	branch, err := c.GetById(ctx, &br.BranchPrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting branch by id")
		return nil, err
	}
	return branch, nil
}

func (c *branchRepo) GetAll(ctx context.Context, req *br.GetListBranchRequest) (*br.GetListBranchResponse, error) {
	branches := br.GetListBranchResponse{}
	var (
		created_at sql.NullString
		updated_at sql.NullString
		open_time  sql.NullString
		close_time sql.NullString
		location   sql.NullString
	)
	filter_by_name := ""
	offest := (req.Offset - 1) * req.Limit
	if req.Search != "" {
		filter_by_name = fmt.Sprintf(`AND branch_name ILIKE '%%%v%%'`, req.Search)
	}
	query := `SELECT
				id,
				branch_name,
				branch_location,
				phone,
				open_time,
				close_time,
				created_at,
				updated_at
			FROM branches
			WHERE TRUE AND deleted_at is null ` + filter_by_name + `
			OFFSET $1 LIMIT $2
`
	rows, err := c.db.Query(ctx, query, offest, req.Limit)

	if err != nil {
		log.Println("error while getting all branchs")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			branch br.GetBranch
		)
		if err = rows.Scan(
			&branch.Id,
			&branch.BranchName,
			&location,
			&branch.Phone,
			&open_time,
			&close_time,
			&created_at,
			&updated_at,
		); err != nil {
			return &branches, err
		}
		branch.BranchLocation = location.String
		branch.OpenTime = pkg.NullStringToString(open_time)
		branch.CloseTime = pkg.NullStringToString(close_time)
		branch.CreatedAt = pkg.NullStringToString(created_at)
		branch.UpdatedAt = pkg.NullStringToString(updated_at)

		branches.Branchs = append(branches.Branchs, &branch)
	}

	err = c.db.QueryRow(ctx, `SELECT count(*) from branches WHERE TRUE AND deleted_at is null `+filter_by_name+``).Scan(&branches.Count)
	if err != nil {
		return &branches, err
	}

	return &branches, nil
}

func (c *branchRepo) GetById(ctx context.Context, id *br.BranchPrimaryKey) (*br.GetBranch, error) {
	var (
		branch     br.GetBranch
		created_at sql.NullString
		updated_at sql.NullString
		open_time  sql.NullString
		close_time sql.NullString
		location   sql.NullString
	)

	query := `SELECT
				id,
				branch_name,
				branch_location,
				phone,
				open_time,
				close_time,
				created_at,
				updated_at
			FROM branches
			WHERE id = $1 AND deleted_at IS NULL`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&branch.Id,
		&branch.BranchName,
		&location,
		&branch.Phone,
		&open_time,
		&close_time,
		&created_at,
		&updated_at); err != nil {
		return &branch, err
	}
	branch.BranchLocation = location.String
	branch.OpenTime = pkg.NullStringToString(open_time)
	branch.CloseTime = pkg.NullStringToString(close_time)
	branch.CreatedAt = pkg.NullStringToString(created_at)
	branch.UpdatedAt = pkg.NullStringToString(updated_at)

	return &branch, nil
}

func (c *branchRepo) Delete(ctx context.Context, id *br.BranchPrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE branches SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}
