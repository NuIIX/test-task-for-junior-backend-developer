package task

import "time"

type Status string
type RecurrenceType string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"

	RecurrenceDaily         RecurrenceType = "daily"
	RecurrenceMonthly       RecurrenceType = "monthly"
	RecurrenceSpecificDates RecurrenceType = "specific_dates"
	RecurrenceEvenDays      RecurrenceType = "even_days"
	RecurrenceOddDays       RecurrenceType = "odd_days"
)

type RecurrenceRule struct {
	Type          RecurrenceType
	IntervalDays  *int
	MonthDay      *int
	SpecificDates []string
}

type Task struct {
	ID            int64
	Title         string
	Description   string
	Status        Status
	ScheduledDate *time.Time
	IsTemplate    bool
	ParentTaskID  *int64
	Rule          *RecurrenceRule
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}

func (r RecurrenceType) Valid() bool {
	switch r {
	case RecurrenceDaily, RecurrenceMonthly, RecurrenceSpecificDates, RecurrenceEvenDays, RecurrenceOddDays:
		return true
	default:
		return false
	}
}
