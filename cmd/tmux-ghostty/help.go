package main

import "strings"

type commandHelp struct {
	Usage   string
	Summary string
}

type commandHelpGroup struct {
	Name     string
	Commands []commandHelp
}

var commandHelpGroups = []commandHelpGroup{
	{
		Name: "Lifecycle",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty help", Summary: "Print detailed help for the CLI."},
			{Usage: "tmux-ghostty version", Summary: "Print build metadata, release repo, install dir, current binary path, and installation method."},
			{Usage: "tmux-ghostty self-update [--check] [--version <tag>]", Summary: "Check for or install a GitHub Release package. macOS only. Disabled for Homebrew installs."},
			{Usage: "tmux-ghostty uninstall", Summary: "Remove installed binaries and runtime data. For direct installs this normally requires sudo."},
		},
	},
	{
		Name: "Broker",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty up", Summary: "Start the local broker if needed and report the broker socket path."},
			{Usage: "tmux-ghostty down [--force]", Summary: "Stop the local broker. Use --force to shut it down even when workspaces are still active."},
			{Usage: "tmux-ghostty status", Summary: "Print broker status as JSON."},
		},
	},
	{
		Name: "Workspace",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty workspace create", Summary: "Create a workspace and its first pane."},
			{Usage: "tmux-ghostty workspace reconcile", Summary: "Rebuild workspace state from the current Ghostty/tmux view."},
			{Usage: "tmux-ghostty workspace close <workspace-id>", Summary: "Close a workspace and all panes that belong to it."},
		},
	},
	{
		Name: "Pane",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty pane list", Summary: "List panes as JSON."},
			{Usage: "tmux-ghostty pane focus <pane-id>", Summary: "Focus the pane in Ghostty."},
			{Usage: "tmux-ghostty pane snapshot <pane-id>", Summary: "Capture pane text and metadata from tmux."},
		},
	},
	{
		Name: "Host",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty host attach <pane-id> <query>", Summary: "Search JumpServer and attach the pane to the selected remote host/session."},
		},
	},
	{
		Name: "Control",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty claim <pane-id> --actor agent|user", Summary: "Give control of the pane to the selected actor."},
			{Usage: "tmux-ghostty release <pane-id>", Summary: "Release control of the pane."},
			{Usage: "tmux-ghostty interrupt <pane-id>", Summary: "Interrupt the running command in the pane."},
			{Usage: "tmux-ghostty observe <pane-id>", Summary: "Put the pane into observe-only mode."},
		},
	},
	{
		Name: "Approvals",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty actions", Summary: "List queued approval actions as JSON."},
			{Usage: "tmux-ghostty approve <action-id>", Summary: "Approve a queued risky command."},
			{Usage: "tmux-ghostty deny <action-id>", Summary: "Deny a queued risky command."},
		},
	},
	{
		Name: "Commands",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty command preview <pane-id> <command...>", Summary: "Classify a command and show whether approval is required before execution."},
			{Usage: "tmux-ghostty command send <pane-id> <command...>", Summary: "Send a command to the pane. Risky commands must be approved first."},
		},
	},
}

var helpNotes = []string{
	"Most workspace, pane, host, control, and command subcommands auto-start the local broker.",
	`Use "tmux-ghostty pane list" to discover pane IDs before focus, snapshot, host, or control operations.`,
	"Most query-style commands print JSON.",
	`Use "tmux-ghostty command preview" before "command send" when you are unsure whether a command is risky.`,
	`Use "tmux-ghostty actions" to inspect pending approvals, then "approve" or "deny" the action ID.`,
}

func usageText() string {
	lines := []string{"Usage:"}
	for _, group := range commandHelpGroups {
		for _, command := range group.Commands {
			lines = append(lines, "  "+command.Usage)
		}
	}
	lines = append(lines, "", `Run "tmux-ghostty help" for detailed command descriptions.`)
	return strings.Join(lines, "\n")
}

func helpText() string {
	var lines []string
	lines = append(lines, "tmux-ghostty", "")
	lines = append(lines, "Ghostty is the visible terminal UI. tmux carries the shared text/session state that both the user and the agent operate on.", "")
	for _, group := range commandHelpGroups {
		lines = append(lines, group.Name+":")
		for _, command := range group.Commands {
			lines = append(lines, "  "+command.Usage)
			lines = append(lines, "      "+command.Summary)
		}
		lines = append(lines, "")
	}
	lines = append(lines, "Notes:")
	for _, note := range helpNotes {
		lines = append(lines, "  - "+note)
	}
	return strings.Join(lines, "\n")
}
