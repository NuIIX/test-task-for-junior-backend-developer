package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	now := s.now()
	model := &taskdomain.Task{
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		Rule:        normalized.Rule,
		IsTemplate:  normalized.Rule != nil,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	if created.IsTemplate {
		_ = s.generateForTemplate(ctx, created, 30)
	}

	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	normalized, err := validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	existing.Title = normalized.Title
	existing.Description = normalized.Description
	existing.Status = normalized.Status
	existing.UpdatedAt = s.now()

	if existing.IsTemplate && normalized.Rule != nil {
		existing.Rule = normalized.Rule
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}

	if updated.IsTemplate {
		_ = s.generateForTemplate(ctx, updated, 30)
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]taskdomain.Task, error) {
	return s.repo.List(ctx)
}

func (s *Service) GenerateRecurringTasks(ctx context.Context, daysAhead int) error {
	templates, err := s.repo.GetTemplates(ctx)
	if err != nil {
		return err
	}
	for _, tmpl := range templates {
		_ = s.generateForTemplate(ctx, &tmpl, daysAhead)
	}
	return nil
}

func (s *Service) generateForTemplate(ctx context.Context, tmpl *taskdomain.Task, daysAhead int) error {
	start := s.now().Truncate(24 * time.Hour)
	end := start.AddDate(0, 0, daysAhead)

	var newTasks []taskdomain.Task
	for d := start; d.Before(end) || d.Equal(end); d = d.AddDate(0, 0, 1) {
		if matchRule(tmpl.Rule, d, tmpl.CreatedAt) {
			schedDate := d
			newTasks = append(newTasks, taskdomain.Task{
				Title:         tmpl.Title,
				Description:   tmpl.Description,
				Status:        taskdomain.StatusNew,
				ScheduledDate: &schedDate,
				ParentTaskID:  &tmpl.ID,
				CreatedAt:     s.now(),
				UpdatedAt:     s.now(),
			})
		}
	}

	return s.repo.CreateBatch(ctx, newTasks)
}

func matchRule(rule *taskdomain.RecurrenceRule, date time.Time, createdAt time.Time) bool {
	if rule == nil {
		return false
	}

	switch rule.Type {
	case taskdomain.RecurrenceDaily:
		if rule.IntervalDays == nil || *rule.IntervalDays <= 0 {
			return false
		}
		daysDiff := int(date.Sub(createdAt.Truncate(24 * time.Hour)).Hours() / 24)
		return daysDiff >= 0 && daysDiff%(*rule.IntervalDays) == 0

	case taskdomain.RecurrenceMonthly:
		if rule.MonthDay == nil {
			return false
		}
		targetDay := *rule.MonthDay
		daysInMonth := time.Date(date.Year(), date.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		if targetDay > daysInMonth {
			targetDay = daysInMonth
		}
		return date.Day() == targetDay

	case taskdomain.RecurrenceEvenDays:
		return date.Day()%2 == 0

	case taskdomain.RecurrenceOddDays:
		return date.Day()%2 != 0

	case taskdomain.RecurrenceSpecificDates:
		dateStr := date.Format(time.DateOnly)
		for _, sd := range rule.SpecificDates {
			if sd == dateStr {
				return true
			}
		}
		return false
	}
	return false
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}
	if !input.Status.Valid() {
		return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}
	return input, nil
}

func validateUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if !input.Status.Valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}
	return input, nil
}
