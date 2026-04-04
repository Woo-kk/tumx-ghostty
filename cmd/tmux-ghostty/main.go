package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Woo-kk/tmux-ghostty/internal/app"
	"github.com/Woo-kk/tmux-ghostty/internal/buildinfo"
	"github.com/Woo-kk/tmux-ghostty/internal/install"
	"github.com/Woo-kk/tmux-ghostty/internal/model"
	"github.com/Woo-kk/tmux-ghostty/internal/rpc"
	"github.com/Woo-kk/tmux-ghostty/internal/update"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) > 0 && args[0] == "serve-broker" {
		if err := app.RunBrokerProcess(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	if len(args) == 0 {
		usage()
		return 1
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printHelp(os.Stdout)
		return 0
	}
	switch args[0] {
	case "version":
		return runVersion(args[1:])
	case "self-update":
		return runSelfUpdate(context.Background(), args[1:])
	case "uninstall":
		return runUninstall(context.Background(), args[1:])
	}

	paths, err := app.DefaultPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	ctx := context.Background()
	switch args[0] {
	case "up":
		client, err := app.EnsureBroker(ctx, paths)
		if err != nil {
			return printError(err)
		}
		var status model.BrokerStatus
		if err := client.Call(ctx, "broker.status", nil, &status); err != nil {
			return printError(err)
		}
		fmt.Printf("broker running: %s\n", paths.SocketPath)
		return 0
	case "down":
		return runDown(ctx, paths, args[1:])
	case "status":
		client, err := app.EnsureBroker(ctx, paths)
		if err != nil {
			return printError(err)
		}
		var status model.BrokerStatus
		if err := client.Call(ctx, "broker.status", nil, &status); err != nil {
			return printError(err)
		}
		printJSON(status)
		return 0
	case "workspace":
		return runWorkspace(ctx, paths, args[1:])
	case "pane":
		return runPane(ctx, paths, args[1:])
	case "host":
		return runHost(ctx, paths, args[1:])
	case "claim":
		return runClaim(ctx, paths, args[1:])
	case "release":
		return runRelease(ctx, paths, args[1:])
	case "interrupt":
		return runInterrupt(ctx, paths, args[1:])
	case "observe":
		return runObserve(ctx, paths, args[1:])
	case "actions":
		return runActions(ctx, paths)
	case "approve":
		return runApprove(ctx, paths, args[1:])
	case "deny":
		return runDeny(ctx, paths, args[1:])
	case "command":
		return runCommand(ctx, paths, args[1:])
	default:
		usage()
		return 1
	}
}

func runVersion(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: tmux-ghostty version")
		return 1
	}
	info := buildinfo.Current()
	currentInstall, _ := install.DetectInstallation()
	printJSON(struct {
		Version            string `json:"version"`
		Commit             string `json:"commit"`
		BuildDate          string `json:"build_date"`
		ReleaseRepo        string `json:"release_repo"`
		PackageID          string `json:"package_id"`
		InstallDir         string `json:"install_dir"`
		CurrentBinary      string `json:"current_binary"`
		InstallationMethod string `json:"installation_method"`
	}{
		Version:            info.Version,
		Commit:             info.Commit,
		BuildDate:          info.BuildDate,
		ReleaseRepo:        install.ReleaseRepo(),
		PackageID:          install.PackageID(),
		InstallDir:         install.InstallDir(),
		CurrentBinary:      currentInstall.ResolvedPath,
		InstallationMethod: string(currentInstall.Method),
	})
	return 0
}

func runSelfUpdate(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("self-update", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	check := flags.Bool("check", false, "check for updates without installing")
	targetVersion := flags.String("version", "", "install a specific release tag")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: tmux-ghostty self-update [--check] [--version <tag>]")
		return 1
	}
	if runtime.GOOS != "darwin" {
		return printError(fmt.Errorf("self-update is only supported on macOS"))
	}
	if currentInstall, err := install.DetectInstallation(); err == nil && currentInstall.Method == install.InstallationMethodHomebrew {
		return printError(fmt.Errorf("self-update is disabled for Homebrew installs; use: brew upgrade %s", install.HomebrewFormulaName()))
	}

	client := update.NewGitHubClient(install.ReleaseRepo())
	var release update.Release
	var err error
	if strings.TrimSpace(*targetVersion) != "" {
		release, err = client.ReleaseByTag(ctx, strings.TrimSpace(*targetVersion))
	} else {
		release, err = client.LatestRelease(ctx)
	}
	if err != nil {
		return printError(err)
	}

	currentVersion := buildinfo.Version
	packageAsset, checksumsAsset, err := update.FindRequiredAssets(release)
	if err != nil {
		return printError(err)
	}

	status := struct {
		CurrentVersion  string `json:"current_version"`
		TargetVersion   string `json:"target_version"`
		UpdateAvailable bool   `json:"update_available"`
		ReleaseRepo     string `json:"release_repo"`
		PackageAsset    string `json:"package_asset"`
	}{
		CurrentVersion:  currentVersion,
		TargetVersion:   release.TagName,
		UpdateAvailable: currentVersion != release.TagName,
		ReleaseRepo:     install.ReleaseRepo(),
		PackageAsset:    packageAsset.Name,
	}
	if *check {
		printJSON(status)
		return 0
	}
	if !status.UpdateAvailable {
		fmt.Printf("already up to date: %s\n", currentVersion)
		return 0
	}

	tempDir, err := os.MkdirTemp("", "tmux-ghostty-update-*")
	if err != nil {
		return printError(err)
	}
	defer os.RemoveAll(tempDir)

	packagePath := filepath.Join(tempDir, packageAsset.Name)
	checksumsPath := filepath.Join(tempDir, checksumsAsset.Name)

	fmt.Printf("downloading %s\n", packageAsset.Name)
	if err := client.DownloadFile(ctx, packageAsset.BrowserDownloadURL, packagePath); err != nil {
		return printError(err)
	}
	if err := client.DownloadFile(ctx, checksumsAsset.BrowserDownloadURL, checksumsPath); err != nil {
		return printError(err)
	}

	checksumsData, err := os.ReadFile(checksumsPath)
	if err != nil {
		return printError(err)
	}
	expectedChecksum, ok := update.ParseChecksums(checksumsData)[packageAsset.Name]
	if !ok {
		return printError(fmt.Errorf("checksums.txt did not include %s", packageAsset.Name))
	}
	if err := update.VerifyChecksum(packagePath, expectedChecksum); err != nil {
		return printError(err)
	}

	fmt.Printf("installing %s from %s\n", release.TagName, install.ReleaseRepo())
	if err := installPackage(packagePath); err != nil {
		return printError(err)
	}
	fmt.Printf("update installed: %s\n", release.TagName)
	return 0
}

func runUninstall(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: tmux-ghostty uninstall")
		return 1
	}
	if currentInstall, err := install.DetectInstallation(); err == nil && currentInstall.Method == install.InstallationMethodHomebrew {
		return printError(fmt.Errorf("Homebrew-managed install detected at %s; use: brew uninstall %s", currentInstall.ResolvedPath, install.HomebrewFormulaName()))
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "uninstall requires administrator privileges; run: sudo tmux-ghostty uninstall")
		return 1
	}

	paths, err := uninstallPaths()
	if err != nil {
		return printError(err)
	}
	if err := stopBrokerForUninstall(ctx, paths); err != nil {
		return printError(err)
	}

	if err := removeIfExists(install.BrokerBinaryPath()); err != nil {
		return printError(err)
	}
	if err := os.RemoveAll(paths.BaseDir); err != nil {
		return printError(err)
	}
	if err := forgetPackageReceipt(); err != nil {
		return printError(err)
	}
	if err := removeIfExists(install.MainBinaryPath()); err != nil {
		return printError(err)
	}

	fmt.Printf("removed: %s\n", install.BrokerBinaryPath())
	fmt.Printf("removed: %s\n", install.MainBinaryPath())
	fmt.Printf("removed runtime data: %s\n", paths.BaseDir)
	fmt.Printf("forgot package receipt: %s\n", install.PackageID())
	return 0
}

func runDown(ctx context.Context, paths app.Paths, args []string) int {
	flags := flag.NewFlagSet("down", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	force := flags.Bool("force", false, "force broker shutdown even with active workspaces")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	client := app.ConnectBroker(paths)
	err := client.Call(ctx, "broker.shutdown", map[string]any{"force": *force}, &struct{}{})
	if err != nil {
		var rpcErr *rpc.RPCError
		if errors.As(err, &rpcErr) && rpcErr.Message == rpc.ReasonBrokerUnavailable {
			fmt.Println("broker not running")
			return 0
		}
		return printError(err)
	}
	fmt.Println("broker stopped")
	return 0
}

func runWorkspace(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	switch args[0] {
	case "create":
		var result any
		if err := client.Call(ctx, "workspace.create", nil, &result); err != nil {
			return printError(err)
		}
		printJSON(result)
		return 0
	case "reconcile":
		var result any
		if err := client.Call(ctx, "workspace.reconcile", nil, &result); err != nil {
			return printError(err)
		}
		printJSON(result)
		return 0
	case "close":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "workspace id is required")
			return 1
		}
		if err := client.Call(ctx, "workspace.close", map[string]any{"workspace_id": args[1]}, &struct{}{}); err != nil {
			return printError(err)
		}
		fmt.Printf("workspace closed: %s\n", args[1])
		return 0
	default:
		usage()
		return 1
	}
}

func runPane(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	switch args[0] {
	case "list":
		var result []model.Pane
		if err := client.Call(ctx, "pane.list", nil, &result); err != nil {
			return printError(err)
		}
		printJSON(result)
		return 0
	case "focus":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "pane id is required")
			return 1
		}
		if err := client.Call(ctx, "pane.focus", map[string]any{"pane_id": args[1]}, &struct{}{}); err != nil {
			return printError(err)
		}
		fmt.Printf("pane focused: %s\n", args[1])
		return 0
	case "snapshot":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "pane id is required")
			return 1
		}
		var snapshot model.PaneSnapshot
		if err := client.Call(ctx, "pane.snapshot", map[string]any{"pane_id": args[1]}, &snapshot); err != nil {
			return printError(err)
		}
		printJSON(snapshot)
		return 0
	default:
		usage()
		return 1
	}
}

func runHost(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) < 1 || args[0] != "attach" || len(args) < 3 {
		usage()
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	query := strings.Join(args[2:], " ")
	var result any
	if err := client.Call(ctx, "host.attach", map[string]any{"pane_id": args[1], "query": query}, &result); err != nil {
		return printError(err)
	}
	printJSON(result)
	return 0
}

func runClaim(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "pane id is required")
		return 1
	}
	flags := flag.NewFlagSet("claim", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	actor := flags.String("actor", "agent", "actor to claim control for")
	if err := flags.Parse(args[1:]); err != nil {
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	var pane model.Pane
	if err := client.Call(ctx, "control.claim", map[string]any{"pane_id": args[0], "actor": *actor}, &pane); err != nil {
		return printError(err)
	}
	printJSON(pane)
	return 0
}

func runRelease(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "pane id is required")
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	var pane model.Pane
	if err := client.Call(ctx, "control.release", map[string]any{"pane_id": args[0]}, &pane); err != nil {
		return printError(err)
	}
	printJSON(pane)
	return 0
}

func runInterrupt(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "pane id is required")
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	var pane model.Pane
	if err := client.Call(ctx, "command.interrupt", map[string]any{"pane_id": args[0]}, &pane); err != nil {
		return printError(err)
	}
	printJSON(pane)
	return 0
}

func runObserve(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "pane id is required")
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	var pane model.Pane
	if err := client.Call(ctx, "control.observe", map[string]any{"pane_id": args[0]}, &pane); err != nil {
		return printError(err)
	}
	printJSON(pane)
	return 0
}

func runActions(ctx context.Context, paths app.Paths) int {
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	var actions []any
	if err := client.Call(ctx, "actions.list", nil, &actions); err != nil {
		return printError(err)
	}
	printJSON(actions)
	return 0
}

func runApprove(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "action id is required")
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	var action any
	if err := client.Call(ctx, "command.approve", map[string]any{"action_id": args[0]}, &action); err != nil {
		return printError(err)
	}
	printJSON(action)
	return 0
}

func runDeny(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "action id is required")
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	var action any
	if err := client.Call(ctx, "command.deny", map[string]any{"action_id": args[0]}, &action); err != nil {
		return printError(err)
	}
	printJSON(action)
	return 0
}

func runCommand(ctx context.Context, paths app.Paths, args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}
	client, err := app.EnsureBroker(ctx, paths)
	if err != nil {
		return printError(err)
	}
	switch args[0] {
	case "preview":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: tmux-ghostty command preview <pane-id> <command...>")
			return 1
		}
		var result any
		if err := client.Call(ctx, "command.preview", map[string]any{
			"pane_id": args[1],
			"actor":   "agent",
			"command": strings.Join(args[2:], " "),
		}, &result); err != nil {
			return printError(err)
		}
		printJSON(result)
		return 0
	case "send":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: tmux-ghostty command send <pane-id> <command...>")
			return 1
		}
		var result any
		if err := client.Call(ctx, "command.send", map[string]any{
			"pane_id": args[1],
			"actor":   "agent",
			"command": strings.Join(args[2:], " "),
		}, &result); err != nil {
			return printError(err)
		}
		printJSON(result)
		return 0
	default:
		usage()
		return 1
	}
}

func usage() {
	printUsage(os.Stderr)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, usageText())
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, helpText())
}

func printJSON(value any) {
	buf, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	fmt.Println(string(buf))
}

func printError(err error) int {
	fmt.Fprintln(os.Stderr, err)
	return 1
}

func installPackage(pkgPath string) error {
	installerPath := "/usr/sbin/installer"
	args := []string{"-pkg", pkgPath, "-target", "/"}

	var cmd *exec.Cmd
	if os.Geteuid() == 0 {
		cmd = exec.Command(installerPath, args...)
	} else {
		sudoPath, err := exec.LookPath("sudo")
		if err != nil {
			return fmt.Errorf("self-update requires sudo or root; install manually with: %s %s", installerPath, strings.Join(args, " "))
		}
		cmd = exec.Command(sudoPath, append([]string{installerPath}, args...)...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func uninstallPaths() (app.Paths, error) {
	if baseDir := strings.TrimSpace(os.Getenv("TMUX_GHOSTTY_HOME")); baseDir != "" {
		return app.NewPaths(baseDir), nil
	}

	homeDir, err := invokingUserHomeDir()
	if err != nil {
		return app.Paths{}, err
	}
	return app.PathsForHomeDir(homeDir), nil
}

func invokingUserHomeDir() (string, error) {
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" {
		account, err := user.Lookup(sudoUser)
		if err == nil && strings.TrimSpace(account.HomeDir) != "" {
			return account.HomeDir, nil
		}
	}
	return os.UserHomeDir()
}

func stopBrokerForUninstall(ctx context.Context, paths app.Paths) error {
	client := app.ConnectBroker(paths)
	err := client.Call(ctx, "broker.shutdown", map[string]any{"force": true}, &struct{}{})
	if err == nil {
		return nil
	}

	var rpcErr *rpc.RPCError
	if errors.As(err, &rpcErr) && rpcErr.Message == rpc.ReasonBrokerUnavailable {
		return terminateBrokerFromPID(paths)
	}
	return err
}

func terminateBrokerFromPID(paths app.Paths) error {
	pid, err := app.ReadPID(paths.PIDPath)
	if err != nil || !app.ProcessAlive(pid) {
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !app.ProcessAlive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("broker process %d did not exit after SIGTERM", pid)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func forgetPackageReceipt() error {
	command := exec.Command("pkgutil", "--forget", install.PackageID())
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "no receipt") || strings.Contains(lower, "did not find") {
		return nil
	}
	return fmt.Errorf("pkgutil --forget %s failed: %s", install.PackageID(), trimmed)
}
