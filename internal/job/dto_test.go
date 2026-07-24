package job_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TSY244/ai-study-tutor/internal/job"
)

func TestJobErrorRetryableClassificationMatchesWorkerCodes(t *testing.T) {
	retryable := []string{
		job.ErrCodeWorkerLost,
		job.ErrCodeMaintenanceLeaseExpired,
		"material_fetch_failed",
		"publish_failed",
		"job_timeout",
		"internal_error",
		"claude_timeout",
	}
	for _, code := range retryable {
		t.Run(code, func(t *testing.T) {
			j := failedDTOJob(code)
			dto := job.ToDTO(&j, nil)
			if dto.Error == nil || !dto.Error.Retryable {
				t.Fatalf("error code %s must be retryable: %+v", code, dto.Error)
			}
		})
	}

	for _, code := range []string{"url_blocked", "transcript_required", "claude_refused"} {
		t.Run(code, func(t *testing.T) {
			j := failedDTOJob(code)
			dto := job.ToDTO(&j, nil)
			if dto.Error == nil || dto.Error.Retryable {
				t.Fatalf("error code %s must not be retryable: %+v", code, dto.Error)
			}
		})
	}

	t.Run("persisted classification overrides ambiguous code", func(t *testing.T) {
		j := failedDTOJob("material_fetch_failed")
		retryable := false
		j.ErrorRetryable = &retryable
		dto := job.ToDTO(&j, nil)
		if dto.Error == nil || dto.Error.Retryable {
			t.Fatalf("persisted non-retryable classification must win: %+v", dto.Error)
		}
	})
}

func TestPublishingProgressKeepsValidatingStreamStatus(t *testing.T) {
	data := job.ToJobStreamData(job.Event{
		JobID:     uuid.New(),
		EventType: job.EventProgress,
		Stage:     "publishing",
		Progress:  95,
		CreatedAt: time.Now().UTC(),
	})
	if data.Status != job.StatusValidating {
		t.Fatalf("publishing stream status = %s, want validating", data.Status)
	}
}

func failedDTOJob(code string) job.Job {
	now := time.Now().UTC()
	detail := "redacted detail"
	return job.Job{
		ID:            uuid.New(),
		Kind:          job.KindLessonImport,
		AggregateType: "lesson",
		AggregateID:   uuid.New(),
		Status:        job.StatusFailed,
		Stage:         job.StatusFailed,
		ErrorCode:     &code,
		ErrorDetail:   &detail,
		MaxAttempts:   3,
		CreatedAt:     now,
		UpdatedAt:     now,
		CompletedAt:   &now,
	}
}
