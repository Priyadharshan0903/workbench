package launch

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestPublicRunPreservesOptionalFieldsWithoutExposingRunnerInternals(t *testing.T) {
	if publicRun(nil) != nil {
		t.Fatal("a project without a run must keep a null summary")
	}

	record := &RunRecord{
		RunID:     "fixture-run",
		StartedAt: "2026-10-26T00:00:00Z",
		Phase:     "starting",
		PID:       123,
		Identity:  "fixture-process-identity",
		CWD:       "/private/runner-folder",
		LogFile:   "private-runner.log",
	}

	assertFields := func(expectedFields []string) map[string]json.RawMessage {
		t.Helper()
		encoded, err := json.Marshal(publicRun(record))
		if err != nil {
			t.Fatal(err)
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(encoded, &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != len(expectedFields) {
			t.Fatal("run summary omitted fields or exposed runner internals", string(encoded))
		}
		for field := range fields {
			if !slices.Contains(expectedFields, field) {
				t.Fatal("unexpected public run field", field)
			}
		}

		return fields
	}

	assertFields([]string{"runId", "startedAt", "phase"})

	exitCode := 0
	record.ExitCode = &exitCode
	record.Phase = "completed"
	record.UpdatedAt = "2026-10-26T00:01:00Z"
	record.FinishedAt = record.UpdatedAt
	fields := assertFields([]string{"runId", "startedAt", "phase", "updatedAt", "finishedAt", "exitCode"})
	if string(fields["exitCode"]) != "0" {
		t.Fatal("successful exit code was omitted or changed", string(fields["exitCode"]))
	}
}
