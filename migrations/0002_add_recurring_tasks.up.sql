ALTER TABLE tasks ADD COLUMN scheduled_date DATE;

ALTER TABLE tasks ADD COLUMN is_template BOOLEAN DEFAULT FALSE;

ALTER TABLE tasks ADD COLUMN parent_task_id BIGINT REFERENCES tasks(id) ON DELETE CASCADE;

CREATE TABLE task_recurrence_rules (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT UNIQUE REFERENCES tasks(id) ON DELETE CASCADE,
    recurrence_type VARCHAR(50) NOT NULL,
    interval_days INT,
    month_day INT,
    specific_dates JSONB
);
