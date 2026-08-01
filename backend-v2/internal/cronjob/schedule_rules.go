package cronjob

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// nextRun 按任务类型和用户时区计算下一次执行时间。
func (s *store) nextRun(ctx context.Context, userID string, input JobInput, from time.Time) (*time.Time, error) {
	if input.TaskType == TaskTypeOnce {
		value, err := time.Parse(time.RFC3339, strings.TrimSpace(input.Schedule))
		if err != nil {
			return nil, fmt.Errorf("%w: once schedule must be RFC3339", ErrInvalidJob)
		}
		return &value, nil
	}
	zone, err := s.settings.LoadTimezone(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user timezone: %w", err)
	}
	if strings.TrimSpace(zone) == "" {
		zone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, fmt.Errorf("%w: unknown timezone %q", ErrInvalidJob, zone)
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	expression, err := parser.Parse(strings.TrimSpace(input.Schedule))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid cron expression", ErrInvalidJob)
	}
	next := expression.Next(from.In(location))
	return &next, nil
}

func validateInput(input JobInput) error {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Prompt) == "" || strings.TrimSpace(input.Schedule) == "" {
		return fmt.Errorf("%w: name, schedule and prompt are required", ErrInvalidJob)
	}
	if input.TaskType != TaskTypeOnce && input.TaskType != TaskTypeRecurring {
		return fmt.Errorf("%w: task type must be once or recurring", ErrInvalidJob)
	}
	return nil
}
