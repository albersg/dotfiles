package tui

import (
	"bytes"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"
)

const trainerOutputWaitTimeout = 2 * time.Second

// waitForTrainerOutput waits until the rendered output satisfies condition.
// teatest.WaitFor owns reading the output stream; the matching transcript is
// retained for golden tests that append it to the final output.
func waitForTrainerOutput(t *testing.T, tm *teatest.TestModel, waitingFor string, condition func([]byte) bool) []byte {
	t.Helper()

	var output []byte
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		if !condition(bts) {
			return false
		}
		output = append(output[:0], bts...)
		return true
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(trainerOutputWaitTimeout))
	return output
}

// waitForNextOutputEvent waits for the next length-positive output event. The
// first event can be terminal initialization alone, so callers that need the
// initial rendered frame should wait for two successive events with
// waitForAnyOutput. The consumed bytes are returned for a later final-output check.
func waitForNextOutputEvent(t *testing.T, tm *teatest.TestModel) []byte {
	t.Helper()
	return waitForTrainerOutput(t, tm, "any output", func(output []byte) bool {
		return len(output) > 0
	})
}

// waitForAnyOutput waits for the first output and the following output event. The
// first can be terminal initialization alone; waiting for another length-positive
// chunk lets the initial frame render without relying on a particular screen label.
// Both consumed chunks are retained so a caller can include them in its output check.
func waitForAnyOutput(t *testing.T, tm *teatest.TestModel) *bytes.Buffer {
	t.Helper()
	seen := &bytes.Buffer{}
	for range 2 {
		seen.Write(waitForNextOutputEvent(t, tm))
	}
	return seen
}

func waitForTrainerText(t *testing.T, tm *teatest.TestModel, waitingFor string, alternatives ...string) []byte {
	t.Helper()
	return waitForTrainerOutput(t, tm, waitingFor, func(output []byte) bool {
		for _, text := range alternatives {
			if bytes.Contains(output, []byte(text)) {
				return true
			}
		}
		return false
	})
}
