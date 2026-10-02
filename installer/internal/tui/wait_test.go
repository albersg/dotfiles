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
