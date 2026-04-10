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
			{Usage: "tmux-ghostty workspace inspect-current", Summary: "Inspect the currently focused Ghostty terminal without launching a new window and report whether it can be adopted into a workspace."},
			{Usage: "tmux-ghostty workspace list-windows", Summary: "List current Ghostty windows, tabs, and terminals as JSON, including whether each terminal is already managed by tmux-ghostty."},
			{Usage: "tmux-ghostty workspace bootstrap-current", Summary: "If the current terminal is a local idle shell outside tmux, start a broker-owned tmux session in place and adopt it into a new current-window workspace."},
			{Usage: "tmux-ghostty workspace split-current --direction up|down|left|right [--claim agent|user]", Summary: "Split the currently focused terminal in place and create the first broker-managed pane of a current-window workspace without opening a new window."},
			{Usage: "tmux-ghostty workspace split-terminal --terminal-id <id> --direction up|down|left|right [--claim agent|user]", Summary: "Split an explicit Ghostty terminal by terminal ID in place and create the first broker-managed pane there, even when that terminal is not focused."},
			{Usage: "tmux-ghostty workspace adopt-current", Summary: "Adopt the currently focused Ghostty terminal into a new workspace without opening a new window. Fail explicitly if the current focus is unsuitable."},
			{Usage: "tmux-ghostty workspace reconcile", Summary: "Rebuild workspace state from the current Ghostty/tmux view."},
			{Usage: "tmux-ghostty workspace close <workspace-id>", Summary: "Close a workspace and all panes that belong to it."},
			{Usage: "tmux-ghostty workspace clear <workspace-id>", Summary: "Clear tmux screen/history and cached snapshots for every pane in the workspace."},
			{Usage: "tmux-ghostty workspace delete <workspace-id>", Summary: "Permanently remove a workspace and all of its panes from broker state. Owned local tmux sessions are terminated."},
		},
	},
	{
		Name: "Pane",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty pane list", Summary: "List panes as JSON."},
			{Usage: "tmux-ghostty pane focus <pane-id>", Summary: "Focus the pane in Ghostty."},
			{Usage: "tmux-ghostty pane clear <pane-id>", Summary: "Clear the pane's tmux screen/history and reset its cached snapshot."},
			{Usage: "tmux-ghostty pane delete <pane-id>", Summary: "Permanently remove the pane from broker state. Owned local tmux sessions are terminated."},
			{Usage: "tmux-ghostty pane snapshot <pane-id>", Summary: "Capture pane text and metadata from tmux."},
			{Usage: "tmux-ghostty pane split <pane-id> --direction up|down|left|right [--claim agent|user]", Summary: "Split an existing pane inside the same workspace and return the new pane as JSON."},
		},
	},
	{
		Name: "Host",
		Commands: []commandHelp{
			{Usage: "tmux-ghostty host attach <pane-id> <query>", Summary: "Attach the pane to a remote target through the configured remote provider. The current built-in provider is JumpServer."},
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
	`Use "tmux-ghostty workspace inspect-current" first when you want to stay in the current Ghostty window.`,
	`If inspect-current reports a local shell outside tmux, run "tmux-ghostty workspace bootstrap-current". If the focused terminal is unsuitable but you still want to stay in that focused window, run "tmux-ghostty workspace split-current --direction ...". If you need another existing Ghostty terminal that is not focused, run "tmux-ghostty workspace list-windows" and then "tmux-ghostty workspace split-terminal --terminal-id ... --direction ...". If it reports an existing tmux pane, use "workspace adopt-current".`,
	`Current-window commands fail explicitly when the focused Ghostty terminal cannot be adopted. They do not auto-open a replacement window.`,
	`"workspace split-current" only targets the currently focused Ghostty terminal. Use "workspace split-terminal" when you already know the target terminal ID from "workspace list-windows".`,
	`Use "tmux-ghostty pane list" to discover pane IDs before focus, snapshot, host, or control operations.`,
	`"workspace close" keeps a closed workspace record. Use "workspace delete" or "pane delete" when you want permanent removal from broker state.`,
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
