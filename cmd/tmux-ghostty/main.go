package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/guyuanshun/tmux-ghostty/internal/app"
	"github.com/guyuanshun/tmux-ghostty/internal/model"
	"github.com/guyuanshun/tmux-ghostty/internal/rpc"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

const usageText = `Usage:
  tmux-ghostty up
  tmux-ghostty down [--force]
  tmux-ghostty status
  tmux-ghostty workspace create
  tmux-ghostty workspace reconcile
  tmux-ghostty workspace close <workspace-id>
  tmux-ghostty pane list
  tmux-ghostty pane focus <pane-id>
  tmux-ghostty pane snapshot <pane-id>
  tmux-ghostty host attach <pane-id> <query>
  tmux-ghostty claim <pane-id> --actor agent|user
  tmux-ghostty release <pane-id>
  tmux-ghostty interrupt <pane-id>
  tmux-ghostty observe <pane-id>
  tmux-ghostty actions
  tmux-ghostty approve <action-id>
  tmux-ghostty deny <action-id>
  tmux-ghostty command preview <pane-id> <command...>
  tmux-ghostty command send <pane-id> <command...>
  tmux-ghostty help`

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
		printUsage(os.Stdout)
		return 0
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
	fmt.Fprintln(w, usageText)
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
