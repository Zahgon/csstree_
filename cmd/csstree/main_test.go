package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildCommand builds the command once for the tests that run it.
func buildCommand(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "csstree")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")

	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the command failed: %v\n%s", err, out)
	}

	return binary
}

type commandRun struct {
	args   []string
	stdin  string
	stdout string
	stderr string
	status int
}

// TestCommand runs the command over a few inputs and checks what it writes and the status it
// ends with.
func TestCommand(t *testing.T) {
	binary := buildCommand(t)

	runs := []commandRun{
		{args: []string{"roundtrip"}, stdin: "a{color:red}", stdout: "a{color:red}"},
		{args: []string{"roundtrip", "--mode", "spec"}, stdin: "a{color : red}", stdout: "a{color:red}"},
		{args: []string{"tokenize"}, stdin: "a", stdout: "0\tident-token\t0\t1\t1\t\"a\"\n# tokens 1\n"},
		{args: []string{"util", "--", "keyword", "-WEBKIT-Foo"}, stdout: "{\n \"basename\": \"foo\",\n \"name\": \"-webkit-foo\",\n \"prefix\": \"-webkit-\",\n \"vendor\": \"-webkit-\",\n \"custom\": false\n}\n"},
		{args: []string{"util", "--", "is-custom-property", "--a"}, stdout: "true\n"},
		{args: []string{"check", "--property-name", "color"}, stdout: "OK\n"},
		{args: []string{"defsyntax", "roundtrip", "[ a | b ]{1,4}"}, stdout: "[ a | b ]{1,4}\n"},
		{args: []string{"find", "--type", "Nope"}, stdin: "a{b:c}", stdout: "# none\n"},
		{args: []string{"nope"}, stderr: "csstree: unknown command nope\n", status: 2},
		{args: []string{"check", "--property-name", "colour"}, status: 1,
			stderr: "ERROR\tSyntaxReferenceError\t\"Unknown property `colour`\"\n| Unknown property `colour`\n"},
		{args: []string{"parse", "--context", "nope"}, stdin: "a{b:c}", status: 1,
			stderr: "ERROR\tError\t\"Unknown context `nope`\"\n| Unknown context `nope`\n"},
	}

	for _, c := range runs {
		cmd := exec.Command(binary, c.args...)
		cmd.Stdin = strings.NewReader(c.stdin)
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		status := 0
		if exit, ok := err.(*exec.ExitError); ok {
			status = exit.ExitCode()
		} else if err != nil {
			t.Fatalf("%v: running the command failed: %v", c.args, err)
		}

		if status != c.status {
			t.Errorf("%v: the status is %d, want %d (stderr %q)", c.args, status, c.status, stderr.String())
		}
		if got := stdout.String(); got != c.stdout {
			t.Errorf("%v: standard output is %q, want %q", c.args, got, c.stdout)
		}
		if got := stderr.String(); got != c.stderr {
			t.Errorf("%v: standard error is %q, want %q", c.args, got, c.stderr)
		}
	}
}

// TestParseJSONShape checks the JSON a parse writes.
func TestParseJSONShape(t *testing.T) {
	binary := buildCommand(t)

	cmd := exec.Command(binary, "parse")
	cmd.Stdin = strings.NewReader("a{color:red}")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running the command failed: %v", err)
	}

	text := string(out)
	for _, want := range []string{`"type": "StyleSheet"`, `"type": "Declaration"`, `"property": "color"`, `"loc": null`} {
		if !strings.Contains(text, want) {
			t.Errorf("the tree does not hold %s:\n%s", want, text)
		}
	}
}
