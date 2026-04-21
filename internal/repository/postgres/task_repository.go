package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const query = `
		INSERT INTO tasks (title, description, status, scheduled_date, is_template, parent_task_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err = tx.QueryRow(ctx, query, task.Title, task.Description, task.Status, task.ScheduledDate, task.IsTemplate, task.ParentTaskID, task.CreatedAt, task.UpdatedAt).Scan(&task.ID)
	if err != nil {
		return nil, err
	}

	if task.Rule != nil {
		datesJSON, _ := json.Marshal(task.Rule.SpecificDates)
		const ruleQuery = `
			INSERT INTO task_recurrence_rules (task_id, recurrence_type, interval_days, month_day, specific_dates)
			VALUES ($1, $2, $3, $4, $5)
		`
		_, err = tx.Exec(ctx, ruleQuery, task.ID, task.Rule.Type, task.Rule.IntervalDays, task.Rule.MonthDay, datesJSON)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return task, nil
}

func (r *Repository) CreateBatch(ctx context.Context, tasks []taskdomain.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	const query = `
		INSERT INTO tasks (title, description, status, scheduled_date, is_template, parent_task_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, false, $5, $6, $7)
		ON CONFLICT (parent_task_id, scheduled_date) DO NOTHING
	`
	for _, t := range tasks {
		batch.Queue(query, t.Title, t.Description, t.Status, t.ScheduledDate, t.ParentTaskID, t.CreatedAt, t.UpdatedAt)
	}

	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	_, err := br.Exec()
	return err
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT t.id, t.title, t.description, t.status, t.scheduled_date, t.is_template, t.parent_task_id, t.created_at, t.updated_at,
		       r.recurrence_type, r.interval_days, r.month_day, r.specific_dates
		FROM tasks t
		LEFT JOIN task_recurrence_rules r ON t.id = r.task_id
		WHERE t.id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)
	return scanTask(row)
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const query = `
		UPDATE tasks SET title = $1, description = $2, status = $3, updated_at = $4
		WHERE id = $5
	`
	_, err = tx.Exec(ctx, query, task.Title, task.Description, task.Status, task.UpdatedAt, task.ID)
	if err != nil {
		return nil, err
	}

	if task.IsTemplate && task.Rule != nil {
		datesJSON, _ := json.Marshal(task.Rule.SpecificDates)
		const ruleQuery = `
			INSERT INTO task_recurrence_rules (task_id, recurrence_type, interval_days, month_day, specific_dates)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (task_id) DO UPDATE
			SET recurrence_type = EXCLUDED.recurrence_type, interval_days = EXCLUDED.interval_days, month_day = EXCLUDED.month_day, specific_dates = EXCLUDED.specific_dates
		`
		_, err = tx.Exec(ctx, ruleQuery, task.ID, task.Rule.Type, task.Rule.IntervalDays, task.Rule.MonthDay, datesJSON)
		if err != nil {
			return nil, err
		}

		const deleteFutureQuery = `DELETE FROM tasks WHERE parent_task_id = $1 AND status = 'new' AND scheduled_date > CURRENT_DATE`
		_, err = tx.Exec(ctx, deleteFutureQuery, task.ID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, task.ID)
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}
	return nil
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT t.id, t.title, t.description, t.status, t.scheduled_date, t.is_template, t.parent_task_id, t.created_at, t.updated_at,
		       r.recurrence_type, r.interval_days, r.month_day, r.specific_dates
		FROM tasks t
		LEFT JOIN task_recurrence_rules r ON t.id = r.task_id
		ORDER BY t.scheduled_date DESC NULLS LAST, t.id DESC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []taskdomain.Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, nil
}

func (r *Repository) GetTemplates(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT t.id, t.title, t.description, t.status, t.scheduled_date, t.is_template, t.parent_task_id, t.created_at, t.updated_at,
		       r.recurrence_type, r.interval_days, r.month_day, r.specific_dates
		FROM tasks t
		JOIN task_recurrence_rules r ON t.id = r.task_id
		WHERE t.is_template = true
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []taskdomain.Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var t taskdomain.Task
	var status string
	var rType *string
	var rInterval, rMonthDay *int
	var rDates []byte

	err := scanner.Scan(
		&t.ID, &t.Title, &t.Description, &status, &t.ScheduledDate, &t.IsTemplate, &t.ParentTaskID, &t.CreatedAt, &t.UpdatedAt,
		&rType, &rInterval, &rMonthDay, &rDates,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}
		return nil, err
	}

	t.Status = taskdomain.Status(status)

	if rType != nil {
		var specificDates []string
		if rDates != nil {
			_ = json.Unmarshal(rDates, &specificDates)
		}
		t.Rule = &taskdomain.RecurrenceRule{
			Type:          taskdomain.RecurrenceType(*rType),
			IntervalDays:  rInterval,
			MonthDay:      rMonthDay,
			SpecificDates: specificDates,
		}
	}

	return &t, nil
}
