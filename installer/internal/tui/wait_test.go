package tui

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"
)

const trainerOutputWaitTimeout = 2 * time.Second

// waitForTrainerOutput waits until the rendered output satisfies condition.
// Keeping the accumulated output lets timeout failures show the evidence that
// the test actually saw instead of reporting only that a wait expired.
func waitForTrainerOutput(t *testing.T, tm *teatest.TestModel, waitingFor string, condition func([]byte) bool) []byte {
	t.Helper()

	var output bytes.Buffer
	reader := tm.Output()
	deadline := time.Now().Add(trainerOutputWaitTimeout)
	for time.Now().Before(deadline) {
		if _, err := io.Copy(&output, reader); err != nil {
			t.Fatalf("failed waiting for %q: reading output: %v; output seen:\n%s", waitingFor, err, output.String())
		}
		if condition(output.Bytes()) {
			return append([]byte(nil), output.Bytes()...)
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %q after %s; output seen:\n%s", waitingFor, trainerOutputWaitTimeout, output.String())
	return nil
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
