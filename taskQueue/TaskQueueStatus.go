package taskQueue

import (
	"fmt"
	"math"
	"time"
)

type TaskQueueStatus struct {
	CurrentQueueLength     int         `json:"currentQueueLength"`
	CurrentQueueByPriority map[int]int `json:"currentQueueByPriority"`

	TotalSuccessful        int         `json:"totalSuccessful"`
	TotalSuccessfulByRetry map[int]int `json:"totalSuccessfulByRetry"`
	TotalFailed            int         `json:"totalFailed"`

	Options                taskQueueOptions `json:"taskQueueOptions"`
	First                  time.Time        `json:"first"`
	Last                   time.Time        `json:"last"`
	MinSuccessDuration     time.Duration    `json:"minSuccessDuration"`
	MaxSuccessDuration     time.Duration    `json:"maxSuccessDuration"`
	AverageSuccessDuration time.Duration    `json:"averageSuccessDuration"`
	MinFailureDuration     time.Duration    `json:"minFailureDuration"`
	MaxFailureDuration     time.Duration    `json:"maxFailureDuration"`
	AverageFailureDuration time.Duration    `json:"averageFailureDuration"`

	Reports []TaskReport `json:"-"`
}

func (s TaskQueueStatus) String() string {
	return fmt.Sprintf("TaskQueue[currentLength: %d, successful: %d, failed: %d]", s.CurrentQueueLength, s.TotalSuccessful, s.TotalFailed)
}

func (s TaskQueueStatus) Pretty() string {
	return fmt.Sprintf(`TaskQueueStatus [
	// IN-FLIGHT
	currentQueueLength: %d

	// PAST
	totalSuccessful:    %d
	totalFailed:        %d

	// LAST %s
	firstMessage:       %s
	lastMessage:        %s
	minSuccessDuration: %s
	maxSuccessDuration: %s
	avgSuccessDuration: %s
	minFailureDuration: %s
	maxFailureDuration: %s
	avgFailureDuration: %s
] `,
		s.CurrentQueueLength,
		s.TotalSuccessful,
		s.TotalFailed,
		s.Options.keepTaskReportsFor,
		s.First.Format(time.RFC3339),
		s.Last.Format(time.RFC3339),
		s.MinSuccessDuration,
		s.MaxSuccessDuration,
		s.AverageSuccessDuration,
		s.MinFailureDuration,
		s.MaxFailureDuration,
		s.AverageFailureDuration,
	)
}

func statusFromTaskQueue(p *TaskQueue) TaskQueueStatus {

	start := time.Now().Add(24 * time.Hour)
	end := time.Now().Add(-p.defaultOpts.keepTaskReportsFor)
	minSuccessDuration := time.Duration(math.MaxInt64)
	maxSuccessDuration := time.Duration(0)
	sumSuccessDuration := time.Duration(0)
	successCount := 0

	minFailureDuration := time.Duration(math.MaxInt64)
	maxFailureDuration := time.Duration(0)
	sumFailureDuration := time.Duration(0)
	failureCount := 0

	reportsRaw := p.reports.GetWithFilter(func(item interface{}) bool { return true })
	reports := []TaskReport{}

	for _, taskReport := range reportsRaw {
		report := taskReport.(TaskReport)
		reports = append(reports, report)
		if report.Time.Before(start) {
			start = report.Time
		}
		if report.Time.After(end) {
			end = report.Time
		}
		if report.Success {
			successCount++
			sumSuccessDuration += report.Duration
			if minSuccessDuration > report.Duration {
				minSuccessDuration = report.Duration
			}
			if maxSuccessDuration < report.Duration {
				maxSuccessDuration = report.Duration
			}
		} else {
			failureCount++
			sumFailureDuration += report.Duration
			if minFailureDuration > report.Duration {
				minFailureDuration = report.Duration
			}
			if maxFailureDuration < report.Duration {
				maxFailureDuration = report.Duration
			}
		}
	}

	currentQueueLength := p.list.Length()
	currentQueueRaw := p.list.GetWithFilter(func(item interface{}) bool { return true })
	currentQueue := map[int]int{}
	for _, item := range currentQueueRaw {
		priority := item.(task).opts.priority
		if amount, ok := currentQueue[priority]; ok {
			currentQueue[priority] = amount + 1
		} else {
			currentQueue[priority] = 1
		}
	}

	status := TaskQueueStatus{
		CurrentQueueLength:     currentQueueLength,
		CurrentQueueByPriority: currentQueue,
		TotalSuccessful:        p.successful,
		TotalSuccessfulByRetry: p.successfulByRetry,
		TotalFailed:            p.failed,
		First:                  start,
		Last:                   end,
		Options:                p.defaultOpts,
		Reports:                reports,
	}

	if successCount > 0 {
		status.AverageSuccessDuration = sumSuccessDuration / time.Duration(successCount)
		status.MinSuccessDuration = minSuccessDuration
		status.MaxSuccessDuration = maxSuccessDuration
	}

	if failureCount > 0 {
		status.AverageFailureDuration = sumFailureDuration / time.Duration(failureCount)
		status.MinFailureDuration = minFailureDuration
		status.MaxFailureDuration = maxFailureDuration
	}

	return status
}
