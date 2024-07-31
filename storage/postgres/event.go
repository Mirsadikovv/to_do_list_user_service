package postgres

import (
	"context"
	"database/sql"
	"fmt"
	br "go_user_service/genproto/event_service"
	"go_user_service/pkg"
	"go_user_service/storage"
	"log"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

type eventRepo struct {
	db *pgxpool.Pool
}

func NewEventRepo(db *pgxpool.Pool) storage.EventRepoI {
	return &eventRepo{
		db: db,
	}
}

func (c *eventRepo) Create(ctx context.Context, req *br.CreateEvent) (*br.GetEvent, error) {
	id := uuid.NewString()

	comtag, err := c.db.Exec(ctx, `
		INSERT INTO events (
			id,
			branch_id,
			topic,
			start_time,
			end_time
		) VALUES ($1,$2,$3,$4,$5
		)`,
		id,
		req.BranchId,
		req.Topic,
		req.StartTime,
		req.EndTime,
	)
	if err != nil {
		log.Println("error while creating event", comtag)
		return nil, err
	}

	event, err := c.GetById(ctx, &br.EventPrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting event by id")
		return nil, err
	}
	return event, nil
}

func (c *eventRepo) Update(ctx context.Context, req *br.UpdateEvent) (*br.GetEvent, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE events SET
		branch_id = $1,
		topic = $2,
		start_time = $3,
		end_time = $4,
		updated_at = NOW()
		WHERE id = $5
		`,
		req.BranchId,
		req.Topic,
		req.StartTime,
		req.EndTime,
		req.Id,
	)
	if err != nil {
		log.Println("error while updating event")
		return nil, err
	}

	event, err := c.GetById(ctx, &br.EventPrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting event by id")
		return nil, err
	}
	return event, nil
}

func (c *eventRepo) GetAll(ctx context.Context, req *br.GetListEventRequest) (*br.GetListEventResponse, error) {
	events := br.GetListEventResponse{}
	var (
		created_at sql.NullString
		updated_at sql.NullString
		start_time sql.NullString
		end_time   sql.NullString
	)
	filter_by_name := ""
	offest := (req.Offset - 1) * req.Limit
	if req.Search != "" {
		filter_by_name = fmt.Sprintf(`AND event_name ILIKE '%%%v%%'`, req.Search)
	}
	query := `SELECT
				id,
				branch_id,
				topic,
				start_time,
				end_time,
				created_at,
				updated_at
			FROM events
			WHERE TRUE AND deleted_at is null ` + filter_by_name + `
			OFFSET $1 LIMIT $2
`
	rows, err := c.db.Query(ctx, query, offest, req.Limit)

	if err != nil {
		log.Println("error while getting all events")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			event br.GetEvent
		)
		if err = rows.Scan(
			&event.Id,
			&event.BranchId,
			&event.Topic,
			&start_time,
			&end_time,
			&created_at,
			&updated_at,
		); err != nil {
			return &events, err
		}
		event.StartTime = pkg.NullStringToString(start_time)
		event.EndTime = pkg.NullStringToString(end_time)
		event.CreatedAt = pkg.NullStringToString(created_at)
		event.UpdatedAt = pkg.NullStringToString(updated_at)

		events.Events = append(events.Events, &event)
	}

	err = c.db.QueryRow(ctx, `SELECT count(*) from events WHERE TRUE AND deleted_at is null `+filter_by_name+``).Scan(&events.Count)
	if err != nil {
		return &events, err
	}

	return &events, nil
}

func (c *eventRepo) GetById(ctx context.Context, id *br.EventPrimaryKey) (*br.GetEvent, error) {
	var (
		event      br.GetEvent
		created_at sql.NullString
		updated_at sql.NullString
		start_time sql.NullString
		end_time   sql.NullString
	)

	query := `SELECT
				id,
				branch_id,
				topic,
				start_time,
				end_time,
				created_at,
				updated_at
			FROM events
			WHERE id = $1 AND deleted_at IS NULL`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&event.Id,
		&event.BranchId,
		&event.Topic,
		&start_time,
		&end_time,
		&created_at,
		&updated_at); err != nil {
		return &event, err
	}
	event.StartTime = pkg.NullStringToString(start_time)
	event.EndTime = pkg.NullStringToString(end_time)
	event.CreatedAt = pkg.NullStringToString(created_at)
	event.UpdatedAt = pkg.NullStringToString(updated_at)

	return &event, nil
}

func (c *eventRepo) Delete(ctx context.Context, id *br.EventPrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE events SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}
