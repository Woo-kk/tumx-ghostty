package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunHelpOutputsUsageToStdout(t *testing.T) {
	testCases := [][]string{
		{"help"},
		{"-h"},
		{"--help"},
	}

	for _, args := range testCases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			stdout, stderr, code := captureRunOutput(t, args)
			if code != 0 {
				t.Fatalf("run(%v) = %d, want 0", args, code)
			}
			if stderr != "" {
				t.Fatalf("expected no stderr for %v, got %q", args, stderr)
			}
			if stdout != usageText+"\n" {
				t.Fatalf("unexpected stdout for %v: %q", args, stdout)
			}
		})
	}
}

func TestRunNoArgsOutputsUsageToStderr(t *testing.T) {
	stdout, stderr, code := captureRunOutput(t, nil)
	if code != 1 {
		t.Fatalf("run(nil) = %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	if stderr != usageText+"\n" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestRunUnknownCommandOutputsUsageToStderr(t *testing.T) {
	stdout, stderr, code := captureRunOutput(t, []string{"unknown"})
	if code != 1 {
		t.Fatalf("run(unknown) = %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	if stderr != usageText+"\n" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func captureRunOutput(t *testing.T, args []string) (string, string, int) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})

	code := run(args)

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()

	stdoutBytes, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderrBytes, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	return string(stdoutBytes), string(stderrBytes), code
}
