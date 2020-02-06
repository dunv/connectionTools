package sendQueue

import (
	"fmt"
	"math"
	"time"
)

type SendQueueStatus struct {
	QueueLength                 int         `json:"queueLength"`
	TotalSuccessfulSends        int         `json:"totalSuccessfulSends"`
	TotalSuccessfulSendsByRetry map[int]int `json:"totalSuccessfulSendsByRetry"`
	TotalFailedSends            int         `json:"totalFailedSends"`
	SendInProgress              bool        `json:"sendInProgress"`

	Options                sendQueueOptions `json:"sendQueueOptions"`
	FirstMessage           time.Time        `json:"firstMessage"`
	LastMessage            time.Time        `json:"lastMessage"`
	MinSuccessDuration     time.Duration    `json:"minSuccessDuration"`
	MaxSuccessDuration     time.Duration    `json:"maxSuccessDuration"`
	AverageSuccessDuration time.Duration    `json:"averageSuccessDuration"`
	MinFailureDuration     time.Duration    `json:"minFailureDuration"`
	MaxFailureDuration     time.Duration    `json:"maxFailureDuration"`
	AverageFailureDuration time.Duration    `json:"averageFailureDuration"`
}

func (s SendQueueStatus) String() string {
	return fmt.Sprintf("SendQueue[length: %d, sendInProgress: %t, successfulSends: %d, failedSends: %d]",
		s.QueueLength, s.SendInProgress, s.TotalSuccessfulSends, s.TotalFailedSends)
}

func (s SendQueueStatus) Pretty() string {
	return fmt.Sprintf(`SendQueueStatus [
	queueLength:          %d
	totalSuccessfulSends: %d
	totalFailedSends:     %d
	sendInProgress:       %t

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
		s.QueueLength,
		s.TotalSuccessfulSends,
		s.TotalFailedSends,
		s.SendInProgress,
		s.Options.keepSendReportsFor,
		s.FirstMessage.Format(time.RFC3339),
		s.LastMessage.Format(time.RFC3339),
		s.MinSuccessDuration,
		s.MaxSuccessDuration,
		s.AverageSuccessDuration,
		s.MinFailureDuration,
		s.MaxFailureDuration,
		s.AverageFailureDuration,
	)
}

func statusFromSendQueue(p *SendQueue) SendQueueStatus {

	start := time.Now().Add(24 * time.Hour)
	end := time.Now().Add(-p.opts.keepSendReportsFor)
	minSuccessDuration := time.Duration(math.MaxInt64)
	maxSuccessDuration := time.Duration(0)
	sumSuccessDuration := time.Duration(0)
	successCount := 0

	minFailureDuration := time.Duration(math.MaxInt64)
	maxFailureDuration := time.Duration(0)
	sumFailureDuration := time.Duration(0)
	failureCount := 0

	for _, sendReport := range p.sendReports.GetWithFilter(func(item interface{}) bool { return true }) {
		report := sendReport.(SendReport)
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

	status := SendQueueStatus{
		QueueLength:                 p.Length(),
		SendInProgress:              p.sendInProgress,
		TotalSuccessfulSends:        p.successfulSends,
		TotalSuccessfulSendsByRetry: p.successfulSendsByRetry,
		TotalFailedSends:            p.failedSends,
		FirstMessage:                start,
		LastMessage:                 end,
		Options:                     p.opts,
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
