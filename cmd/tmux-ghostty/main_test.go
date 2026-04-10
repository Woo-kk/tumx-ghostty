package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Woo-kk/tmux-ghostty/internal/buildinfo"
	"github.com/Woo-kk/tmux-ghostty/internal/install"
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
			if stdout != helpText()+"\n" {
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
	if stderr != usageText()+"\n" {
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
	if stderr != usageText()+"\n" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestHelpTextIncludesCommandGroupsAndKeyCommands(t *testing.T) {
	output := helpText()
	for _, fragment := range []string{
		"Lifecycle:",
		"Broker:",
		"Workspace:",
		"Pane:",
		"Host:",
		"Control:",
		"Approvals:",
		"Commands:",
		"tmux-ghostty workspace create",
		"tmux-ghostty workspace bootstrap-current",
		"tmux-ghostty workspace list-windows",
		"tmux-ghostty workspace split-current --direction up|down|left|right [--claim agent|user]",
		"tmux-ghostty workspace split-terminal --terminal-id <id> --direction up|down|left|right [--claim agent|user]",
		"tmux-ghostty workspace adopt-current",
		"tmux-ghostty workspace clear <workspace-id>",
		"tmux-ghostty workspace delete <workspace-id>",
		"tmux-ghostty pane clear <pane-id>",
		"tmux-ghostty pane delete <pane-id>",
		"tmux-ghostty pane split <pane-id> --direction up|down|left|right [--claim agent|user]",
		"tmux-ghostty command send <pane-id> <command...>",
		"Notes:",
	} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("help text missing %q", fragment)
		}
	}
}

func TestRunVersionOutputsBuildInfo(t *testing.T) {
	stdout, stderr, code := captureRunOutput(t, []string{"version"})
	if code != 0 {
		t.Fatalf("run(version) = %d, want 0", code)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}

	var got struct {
		Version     string `json:"version"`
		Commit      string `json:"commit"`
		BuildDate   string `json:"build_date"`
		ReleaseRepo string `json:"release_repo"`
		PackageID   string `json:"package_id"`
		InstallDir  string `json:"install_dir"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal version output: %v", err)
	}

	info := buildinfo.Current()
	if got.Version != info.Version || got.Commit != info.Commit || got.BuildDate != info.BuildDate {
		t.Fatalf("unexpected version output: %#v", got)
	}
	if got.ReleaseRepo != install.ReleaseRepo() {
		t.Fatalf("unexpected release repo: %#v", got)
	}
	if got.PackageID != install.PackageID() {
		t.Fatalf("unexpected package id: %#v", got)
	}
	if got.InstallDir != install.InstallDir() {
		t.Fatalf("unexpected install dir: %#v", got)
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
