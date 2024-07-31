package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	br "go_user_service/genproto/event_registrate_service"
	"go_user_service/pkg"
	"go_user_service/pkg/check"
	"go_user_service/storage"
	"log"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
)

type eventRegistrateRepo struct {
	db *pgxpool.Pool
}

func NewEventRegistrateRepo(db *pgxpool.Pool) storage.EventRegistrateRepoI {
	return &eventRegistrateRepo{
		db: db,
	}
}
func isStudentRegisteredInOtherBranch(db *pgxpool.Pool, ctx context.Context, studentID, branchID, startTime string) (bool, error) {
	var count int
	eventStartTime, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return false, fmt.Errorf("invalid startTime format: %v", err)
	}

	query := `
		SELECT COUNT(*)
		FROM event_registrate er
		JOIN events e ON er.event_id = e.id
		WHERE er.student_id = $1
		AND e.branch_id <> $2
		AND e.start_time::date = $3::date
		`

	err = db.QueryRow(ctx, query, studentID, branchID, eventStartTime).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (c *eventRegistrateRepo) Create(ctx context.Context, req *br.CreateEventRegistrate) (*br.GetEventRegistrate, error) {
	var start_time sql.NullString
	var branch string
	id := uuid.NewString()
	query := `
			SELECT
			start_time
		FROM events
		WHERE id = $1 AND deleted_at is null`

	rows := c.db.QueryRow(ctx, query, req.EventId)

	if err := rows.Scan(
		&start_time); err != nil {
		return nil, err
	}
	hoursUntil, err := check.CheckDeadline(pkg.NullStringToString(start_time))
	if hoursUntil-5 < 3.0 {
		log.Println("error while creating event registrate", err)
		return nil, errors.New("less than 3 hours left before the event starts")
	}

	query1 := `
			SELECT
			branch_id
		FROM events
		WHERE id = $1 AND deleted_at is null`

	rows2 := c.db.QueryRow(ctx, query1, req.EventId)

	if err := rows2.Scan(
		&branch); err != nil {
		return nil, err
	}

	eventStart := pkg.NullStringToString(start_time)

	_, err = isStudentRegisteredInOtherBranch(c.db, ctx, req.StudentId, branch, eventStart)
	if err != nil {
		log.Println("error while registration to event", err)
	} else {
		log.Println("registration success")
	}

	comtag, err := c.db.Exec(ctx, `
		INSERT INTO event_registrate (
			id,
			event_id,
			student_id
		) VALUES ($1,$2,$3
		)`,
		id,
		req.EventId,
		req.StudentId,
	)

	if err != nil {
		log.Println("error while creating event", comtag)
		return nil, err
	}

	event, err := c.GetById(ctx, &br.EventRegistratePrimaryKey{Id: id})
	if err != nil {
		log.Println("error while getting event by id")
		return nil, err
	}
	return event, nil
}

func (c *eventRegistrateRepo) Update(ctx context.Context, req *br.UpdateEventRegistrate) (*br.GetEventRegistrate, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE event_registrate SET
		event_id = $1,
		student_id = $2,
		updated_at = NOW()
		WHERE id = $3
		`,
		req.EventId,
		req.StudentId,
		req.Id,
	)
	if err != nil {
		log.Println("error while updating event_registrate")
		return nil, err
	}

	event_registrate, err := c.GetById(ctx, &br.EventRegistratePrimaryKey{Id: req.Id})
	if err != nil {
		log.Println("error while getting event_registrate by id")
		return nil, err
	}
	return event_registrate, nil
}

func (c *eventRegistrateRepo) GetById(ctx context.Context, id *br.EventRegistratePrimaryKey) (*br.GetEventRegistrate, error) {
	var (
		event_registrate br.GetEventRegistrate
	)

	query := `SELECT
				id,
				event_id,
				student_id
			FROM event_registrate
			WHERE id = $1`

	rows := c.db.QueryRow(ctx, query, id.Id)

	if err := rows.Scan(
		&event_registrate.Id,
		&event_registrate.EventId,
		&event_registrate.StudentId,
	); err != nil {
		return &event_registrate, err
	}

	return &event_registrate, nil
}

func (c *eventRegistrateRepo) Delete(ctx context.Context, id *br.EventRegistratePrimaryKey) (emptypb.Empty, error) {

	_, err := c.db.Exec(ctx, `
		UPDATE event_registrate SET
		deleted_at = NOW()
		WHERE id = $1
		`,
		id.Id)

	if err != nil {
		return emptypb.Empty{}, err
	}
	return emptypb.Empty{}, nil
}

func (c *eventRegistrateRepo) GetStudentEvent(ctx context.Context, req *br.GetListEventRegistrateRequest) (*br.GetListEventRegistrateResponse, error) {
	events := br.GetListEventRegistrateResponse{}
	var (
		start_time sql.NullString
		end_time   sql.NullString
	)

	query := `SELECT
				e.topic,
				e.start_time,
				e.end_time,
				e.branch_id
			FROM events e
			JOIN event_registrate er ON er.event_id =e.id
			WHERE er.student_id = $1 AND e.deleted_at is null 
`
	rows, err := c.db.Query(ctx, query, req.Search)

	if err != nil {
		log.Println("error while getting registrated events")
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			event br.GetStudentEventRegistrateResponse
		)
		if err = rows.Scan(
			&event.Topic,
			&start_time,
			&end_time,
			&event.BranchId,
		); err != nil {
			return &events, err
		}
		event.StartTime = pkg.NullStringToString(start_time)
		event.EndTime = pkg.NullStringToString(end_time)

		events.Events = append(events.Events, &event)
	}

	err = c.db.QueryRow(ctx, `  SELECT count(*) FROM events e
								JOIN event_registrate er ON er.event_id = e.id
								WHERE er.student_id = $1 AND e.deleted_at is null `, req.Search).Scan(&events.Count)
	if err != nil {
		return &events, err
	}

	return &events, nil
}
