package actionlint

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCommandMain(t *testing.T) {
	var output bytes.Buffer

	// Create command instance populating stdin/stdout/stderr
	cmd := Command{
		Stdin:  os.Stdin,
		Stdout: &output,
		Stderr: &output,
	}

	// Run the command end-to-end. Note that given args should contain program name
	workflow := filepath.Join("testdata", "examples", "main.yaml")
	status := cmd.Main([]string{"actionlint", "-shellcheck=", "-pyflakes=", "-ignore", `label .+ is unknown\.`, workflow})

	if status != 1 {
		t.Fatal("exit status should be 1 but got", status)
	}

	out := output.String()

	for _, s := range []string{
		"main.yaml:3:5:",
		"unexpected key \"branch\" for \"push\" section",
		"^~~~~~~~~~~~~~~",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("output should contain %q: %q", s, out)
		}
	}

	if strings.Contains(out, "[runner-label]") {
		t.Errorf("runner-label rule should be ignored by -ignore but it is included in output: %q", out)
	}
}

// reSummaryLine matches a line printed by -summary option
var reSummaryLine = regexp.MustCompile(`(?m)^(\d+ errors? in \d+ files?|no errors)$`)

func TestCommandMainSummary(t *testing.T) {
	mainWorkflow := filepath.Join("testdata", "examples", "main.yaml")
	missingOn := filepath.Join("testdata", "err", "missing_on.yaml")
	emptyOn := filepath.Join("testdata", "err", "empty_on.yaml")
	clean := filepath.Join("testdata", "ok", "anchors.yaml")

	stdin, err := os.ReadFile(missingOn)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		what   string
		args   []string
		stdin  string
		status int
		stderr string
	}{
		{
			what:   "errors in one file",
			args:   []string{"-summary", mainWorkflow},
			status: ExitStatusSuccessProblemFound,
			stderr: "7 errors in 1 file\n",
		},
		{
			what:   "errors in several files",
			args:   []string{"-summary", missingOn, emptyOn},
			status: ExitStatusSuccessProblemFound,
			stderr: "2 errors in 2 files\n",
		},
		{
			what:   "one error",
			args:   []string{"-summary", missingOn},
			status: ExitStatusSuccessProblemFound,
			stderr: "1 error in 1 file\n",
		},
		{
			what:   "no error",
			args:   []string{"-summary", clean},
			status: ExitStatusSuccessNoProblem,
			stderr: "no errors\n",
		},
		{
			what:   "with -oneline",
			args:   []string{"-oneline", "-summary", mainWorkflow},
			status: ExitStatusSuccessProblemFound,
			stderr: "7 errors in 1 file\n",
		},
		{
			what:   "stdin",
			args:   []string{"-summary", "-"},
			stdin:  string(stdin),
			status: ExitStatusSuccessProblemFound,
			stderr: "1 error in 1 file\n",
		},
		{
			what:   "without -summary",
			args:   []string{mainWorkflow},
			status: ExitStatusSuccessProblemFound,
			stderr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.what, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := Command{
				Stdin:  strings.NewReader(tc.stdin),
				Stdout: &stdout,
				Stderr: &stderr,
			}
			args := append([]string{"actionlint", "-shellcheck=", "-pyflakes="}, tc.args...)
			status := cmd.Main(args)
			if status != tc.status {
				t.Errorf("exit status should be %d but got %d. stderr: %q", tc.status, status, stderr.String())
			}
			if have := stderr.String(); have != tc.stderr {
				t.Errorf("stderr should be %q but got %q", tc.stderr, have)
			}
			if tc.status == ExitStatusSuccessProblemFound && stdout.Len() == 0 {
				t.Error("errors should be printed to stdout")
			}
			if reSummaryLine.MatchString(stdout.String()) {
				t.Errorf("summary should not be printed to stdout: %q", stdout.String())
			}
		})
	}
}

func TestCommandMainSummaryKeepsJSONOutputClean(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := Command{
		Stdin:  os.Stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	}
	workflow := filepath.Join("testdata", "examples", "main.yaml")
	status := cmd.Main([]string{"actionlint", "-shellcheck=", "-pyflakes=", "-format", "{{json .}}", "-summary", workflow})
	if status != ExitStatusSuccessProblemFound {
		t.Fatalf("exit status should be %d but got %d. stderr: %q", ExitStatusSuccessProblemFound, status, stderr.String())
	}

	var errs []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &errs); err != nil {
		t.Fatalf("stdout should be valid JSON: %v: %q", err, stdout.String())
	}
	if len(errs) != 7 {
		t.Errorf("JSON output should have 7 errors but got %d", len(errs))
	}
	if have, want := stderr.String(), "7 errors in 1 file\n"; have != want {
		t.Errorf("stderr should be %q but got %q", want, have)
	}
}

func TestCommandMainSummaryNotPrintedOnFailure(t *testing.T) {
	testCases := []struct {
		what   string
		args   []string
		status int
	}{
		{
			what:   "invalid option",
			args:   []string{"-summary", "-unknown-option"},
			status: ExitStatusInvalidCommandOption,
		},
		{
			what:   "fatal error",
			args:   []string{"-summary", filepath.Join("testdata", "this-file-does-not-exist.yaml")},
			status: ExitStatusFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.what, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := Command{
				Stdin:  os.Stdin,
				Stdout: &stdout,
				Stderr: &stderr,
			}
			status := cmd.Main(append([]string{"actionlint", "-shellcheck=", "-pyflakes="}, tc.args...))
			if status != tc.status {
				t.Errorf("exit status should be %d but got %d. stderr: %q", tc.status, status, stderr.String())
			}
			out := stderr.String()
			if reSummaryLine.MatchString(out) {
				t.Errorf("summary should not be printed on failure: %q", out)
			}
		})
	}
}

func TestSummarizeErrors(t *testing.T) {
	testCases := []struct {
		files []string
		want  string
	}{
		{nil, "no errors"},
		{[]string{"a.yaml"}, "1 error in 1 file"},
		{[]string{"a.yaml", "a.yaml"}, "2 errors in 1 file"},
		{[]string{"a.yaml", "b.yaml", "a.yaml"}, "3 errors in 2 files"},
	}

	for _, tc := range testCases {
		errs := make([]*Error, 0, len(tc.files))
		for _, f := range tc.files {
			errs = append(errs, &Error{Filepath: f})
		}
		if have := summarizeErrors(errs); have != tc.want {
			t.Errorf("summary of %v should be %q but got %q", tc.files, tc.want, have)
		}
	}
}
