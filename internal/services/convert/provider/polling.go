package provider

import (
	"fmt"
	"time"
)

func WaitForJob(jobID string, providers []Provider) (*ConvertResult, error) {
	p := ResolveProvider(providers, jobID)
	if p == nil {
		return nil, fmt.Errorf("unknown provider for job: %s", jobID)
	}

	ticker := time.NewTicker(2 * time.Second)
	timeout := time.After(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return &ConvertResult{
				JobID:   jobID,
				Status:  "processing",
				Message: "Job is still processing. Use job_id to check status.",
			}, nil
		case <-ticker.C:
			result, err := p.Status(jobID)
			if err != nil {
				return nil, err
			}
			if result.Status == "finished" || result.Status == "error" {
				return result, nil
			}
		}
	}
}
