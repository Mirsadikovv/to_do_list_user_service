package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"go_user_service/genproto/group_service"
	"go_user_service/pkg"
	"go_user_service/storage"
	"log"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

type groupRepo struct {
	db *pgxpool.Pool
}

func NewGroupRepo(db *pgxpool.Pool) storage.GroupRepoI {
	return &groupRepo{
		db: db,
	}
}

func (c *groupRepo) Create(ctx context.Context, req *group_service.CreateGroup) (*group_service.GetGroup, error) {
	id := uuid.NewString()

	comtag, err := c.db.Exec(ctx, `
		INSERT INTO groups (
			id,
			branch_id,
			teacher_id,
			support_teacher_id,
			group_name,
			group_level,
			started_at,
			finished_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8
		)`,
		id,
		req.BranchId,
		req.TeacherId,
		req.SupportTeacherId,
		req.GroupName,
		req.GroupLevel,
		req.StartedAt,
		req.FinishedAt,
	)
	if err != nil {
		log.Println("error while creating group", comtag)
		return nil, err
	}

	group, err := c.GetById(ctx, &group_service.GroupPrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting group by id")
		return nil, err
	}
	return group, nil
}

func (c *groupRepo) Update(ctx context.Context, req *group_service.UpdateGroup) (*group_service.GetGroup, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE groups SET
		branch_id = $1,
		teacher_id = $2,
		support_teacher_id = $3,
		group_name = $4,
		group_level = $5,
		started_at = $6,
		finished_at = $7,
		updated_at = NOW()
		WHERE id = $8
		`,
		req.BranchId,
		req.TeacherId,
		req.SupportTeacherId,
		req.GroupName,
		req.GroupLevel,
		req.StartedAt,
		req.FinishedAt,
		req.Id,
	)
	if err != nil {
		log.Println("error while updating group")
		return nil, err
	}

	group, err := c.GetById(ctx, &group_service.GroupPrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting group by id")
		return nil, err
	}
	return group, nil
}

func (c *groupRepo) GetAll(ctx context.Context, req *group_service.GetListGroupRequest) (*group_service.GetListGroupResponse, error) {
	groups := group_service.GetListGroupResponse{}
	var (
		created_at  sql.NullString
		updated_at  sql.NullString
		started_at  sql.NullString
		finished_at sql.NullString
	)
	filter_by_name := ""
	offest := (req.Offset - 1) * req.Limit
	if req.Search != "" {
		filter_by_name = fmt.Sprintf(`AND group_name ILIKE '%%%v%%'`, req.Search)
	}
	query := `SELECT
				id,
				branch_id,
				teacher_id,
				support_teacher_id,
				group_name,
				group_level,
				started_at,
				finished_at,
				created_at,
				updated_at
			FROM groups
			WHERE TRUE AND deleted_at is null ` + filter_by_name + `
			OFFSET $1 LIMIT $2
`
	rows, err := c.db.Query(ctx, query, offest, req.Limit)

	if err != nil {
		log.Println("error while getting all groups")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			group group_service.GetGroup
		)
		if err = rows.Scan(
			&group.Id,
			&group.BranchId,
			&group.TeacherId,
			&group.SupportTeacherId,
			&group.GroupName,
			&group.GroupLevel,
			&started_at,
			&finished_at,
			&created_at,
			&updated_at,
		); err != nil {
			return &groups, err
		}
		group.StartedAt = pkg.NullStringToString(started_at)
		group.FinishedAt = pkg.NullStringToString(finished_at)
		group.CreatedAt = pkg.NullStringToString(created_at)
		group.UpdatedAt = pkg.NullStringToString(updated_at)

		groups.Groups = append(groups.Groups, &group)
	}

	err = c.db.QueryRow(ctx, `SELECT count(*) from groups WHERE TRUE AND deleted_at is null `+filter_by_name+``).Scan(&groups.Count)
	if err != nil {
		return &groups, err
	}

	return &groups, nil
}

func (c *groupRepo) GetById(ctx context.Context, id *group_service.GroupPrimaryKey) (*group_service.GetGroup, error) {
	var (
		group       group_service.GetGroup
		created_at  sql.NullString
		updated_at  sql.NullString
		started_at  sql.NullString
		finished_at sql.NullString
	)

	query := `SELECT
				id,
				branch_id,
				teacher_id,
				support_teacher_id,
				group_name,
				group_level,
				started_at,
				finished_at,
				created_at,
				updated_at
			FROM groups
			WHERE id = $1 AND deleted_at IS NULL`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&group.Id,
		&group.BranchId,
		&group.TeacherId,
		&group.SupportTeacherId,
		&group.GroupName,
		&group.GroupLevel,
		&started_at,
		&finished_at,
		&created_at,
		&updated_at); err != nil {
		return &group, err
	}
	group.StartedAt = pkg.NullStringToString(started_at)
	group.FinishedAt = pkg.NullStringToString(finished_at)
	group.CreatedAt = pkg.NullStringToString(created_at)
	group.UpdatedAt = pkg.NullStringToString(updated_at)

	return &group, nil
}

func (c *groupRepo) Delete(ctx context.Context, id *group_service.GroupPrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE groups SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}

func (c *groupRepo) Check(ctx context.Context, id *group_service.GroupPrimaryKey) (*group_service.CheckGroupResp, error) {
	query := `SELECT EXISTS (
                SELECT 1
                FROM groups
                WHERE id = $1 AND deleted_at IS NULL
            )`

	var exists bool
	err := c.db.QueryRow(ctx, query, id.Id).Scan(&exists)
	if err != nil {
		return nil, err
	}

	resp := &group_service.CheckGroupResp{
		Check: exists,
	}

	return resp, nil
}

func (c *groupRepo) GetTBS(ctx context.Context, id *group_service.GroupPrimaryKey) (*group_service.GetTBSresp, error) {
	var (
		group group_service.GetTBSresp
	)
	query := `SELECT
				g.branch_id,
				b.branch_name,
				g.teacher_id,
				t.fullname,
				g.support_teacher_id,
				st.fullname,
				g.group_name
			FROM groups g
			JOIN branches b ON g.branch_id = b.id
			JOIN teachers t ON g.teacher_id = t.id
			JOIN support_teachers st ON g.support_teacher_id = st.id
			WHERE g.id = $1 AND g.deleted_at IS NULL
`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&group.BranchId,
		&group.BranchName,
		&group.TeacherId,
		&group.TeacherName,
		&group.SupportTeacherId,
		&group.SupportTeacherName,
		&group.GroupName); err != nil {
		return &group, err
	}

	query1 := `
			SELECT
			count(*)
		FROM students
		WHERE group_id = $1 AND deleted_at is null`

	rows1 := c.db.QueryRow(ctx, query1, id.Id)

	if err := rows1.Scan(
		&group.StudentCount); err != nil {
		return nil, err
	}

	return &group, nil
}
