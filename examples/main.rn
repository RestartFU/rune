package main

import (
	"fmt"
	"time"
)

type Pipeline enum {
	Queued { ID string },
	Running { ID string, Service string, Attempts int },
	Backoff { ID string, Service string, RetryAfter time.Time },
	Succeeded { ID string, Artifact string },
	Failed { ID string, Reason string },
}

func main() {
	requests := []Pipeline{
		Queued{ID: "build-api"},
		Queued{ID: "build-worker"},
		Queued{ID: "build-web"},
	}

	for _, request := range requests {
		fmt.Printf("request: %s\n", requestID(request))
		final := runPipeline(request)
		fmt.Printf("  final: %s\n\n", requestSummary(final))
	}
}

func runPipeline(request Pipeline) Pipeline {
	for step := 0; step < 6; step++ {
		if isTerminal(request) {
			return request
		}

		next := nextState(request)
		fmt.Printf("  step %d: %s -> %s\n", step+1, requestSummary(request), requestSummary(next))
		request = next
	}

	return request
}

func nextState(request Pipeline) Pipeline {
	switch state := request {
	case Queued:
		return NewPipelineRunning(state.ID, "api", 0)

	case Running:
		if state.Attempts == 0 {
			return NewPipelineBackoff(state.ID, state.Service, time.Now().Add(-1*time.Millisecond))
		}
		if state.Attempts == 1 {
			return NewPipelineSucceeded(state.ID, "artifact-"+state.Service+"-"+state.ID+".tar.gz")
		}
		return NewPipelineFailed(state.ID, "pipeline did not stabilize")

	case Backoff:
		if state.RetryAfter.Before(time.Now()) {
			return NewPipelineRunning(state.ID, state.Service, 1)
		}
		return NewPipelineBackoff(state.ID, state.Service, state.RetryAfter)

	case Succeeded:
		return request

	case Failed:
		return request
	}

	return request
}

func isTerminal(request Pipeline) bool {
	switch request {
	case Succeeded, Failed:
		return true
	default:
		return false
	}
}

func requestID(request Pipeline) string {
	switch value := request {
	case Queued:
		return value.ID
	case Running:
		return value.ID
	case Backoff:
		return value.ID
	case Succeeded:
		return value.ID
	case Failed:
		return value.Reason
	default:
		return "unknown"
	}
}

func requestSummary(request Pipeline) string {
	switch requestValue := request {
	case Queued:
		return "queued " + requestValue.ID
	case Running:
		return fmt.Sprintf("running %s attempt=%d", requestValue.Service, requestValue.Attempts)
	case Backoff:
		return fmt.Sprintf("backoff %s until %s", requestValue.ID, requestValue.RetryAfter.Format(time.RFC3339Nano))
	case Succeeded:
		return "succeeded " + requestValue.Artifact
	case Failed:
		return "failed " + requestValue.Reason
	default:
		return "unknown"
	}
}
