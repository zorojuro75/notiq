package queue

import (
	"testing"

	"github.com/zorojuro75/notiq/internal/domain/entity"
)

func TestTaskTypeForJob(t *testing.T) {
	cases := []struct {
		in   entity.JobType
		want string
	}{
		{entity.JobTypeEmail, TypeEmail},
		{entity.JobTypeSMS, TypeSMS},
		{entity.JobTypeWebhook, TypeWebhook},
		{entity.JobTypeReport, TypeReport},
		{entity.JobType("unknown"), "unknown"}, // falls through to the raw string
	}
	for _, c := range cases {
		if got := TaskTypeForJob(c.in); got != c.want {
			t.Errorf("TaskTypeForJob(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
