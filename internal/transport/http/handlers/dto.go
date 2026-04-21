package handlers

import (
	"time"
	taskdomain "example.com/taskservice/internal/domain/task"
)

type recurrenceRuleDTO struct {
	Type          string   `json:"type"`
	IntervalDays  *int     `json:"interval_days,omitempty"`
	MonthDay      *int     `json:"month_day,omitempty"`
	SpecificDates []string `json:"specific_dates,omitempty"`
}

type taskMutationDTO struct {
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Status      taskdomain.Status  `json:"status"`
	Rule        *recurrenceRuleDTO `json:"recurrence_rule,omitempty"`
}

type taskDTO struct {
	ID            int64              `json:"id"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	Status        taskdomain.Status  `json:"status"`
	ScheduledDate string             `json:"scheduled_date,omitempty"`
	IsTemplate    bool               `json:"is_template"`
	ParentTaskID  *int64             `json:"parent_task_id,omitempty"`
	Rule          *recurrenceRuleDTO `json:"recurrence_rule,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	dto := taskDTO{
		ID:           task.ID,
		Title:        task.Title,
		Description:  task.Description,
		Status:       task.Status,
		IsTemplate:   task.IsTemplate,
		ParentTaskID: task.ParentTaskID,
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}

	if task.ScheduledDate != nil {
		dto.ScheduledDate = task.ScheduledDate.Format(time.DateOnly)
	}

	if task.Rule != nil {
		dto.Rule = &recurrenceRuleDTO{
			Type:          string(task.Rule.Type),
			IntervalDays:  task.Rule.IntervalDays,
			MonthDay:      task.Rule.MonthDay,
			SpecificDates: task.Rule.SpecificDates,
		}
	}
	return dto
}

func mapRuleToDomain(dto *recurrenceRuleDTO) *taskdomain.RecurrenceRule {
	if dto == nil {
		return nil
	}
	return &taskdomain.RecurrenceRule{
		Type:          taskdomain.RecurrenceType(dto.Type),
		IntervalDays:  dto.IntervalDays,
		MonthDay:      dto.MonthDay,
		SpecificDates: dto.SpecificDates,
	}
}
