package broker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Woo-kk/tmux-ghostty/internal/execx"
	"github.com/Woo-kk/tmux-ghostty/internal/ghostty"
	"github.com/Woo-kk/tmux-ghostty/internal/jump"
	"github.com/Woo-kk/tmux-ghostty/internal/logx"
	"github.com/Woo-kk/tmux-ghostty/internal/model"
	"github.com/Woo-kk/tmux-ghostty/internal/rpc"
	"github.com/Woo-kk/tmux-ghostty/internal/tmux"
)

type fakeGhosttyClient struct {
	mu              sync.Mutex
	windowCounter   int
	tabCounter      int
	terminalCounter int
	splitErr        error
	inputTextHook   func(string, string) error
	sendKeyHook     func(string, string, []string) error
	inputTexts      []fakeGhosttyInput
	sendKeyCalls    []fakeGhosttyKey
	windows         map[string]ghostty.WindowRef
	tabs            map[string][]ghostty.TabRef
	terminals       map[string][]ghostty.TerminalRef
	focus           ghostty.FocusContext
}

type fakeGhosttyInput struct {
	terminalID string
	text       string
}

type fakeGhosttyKey struct {
	terminalID string
	key        string
	modifiers  []string
}

type fakeJumpClient struct{}

func newFakeGhosttyClient() *fakeGhosttyClient {
	return &fakeGhosttyClient{
		windows:   map[string]ghostty.WindowRef{},
		tabs:      map[string][]ghostty.TabRef{},
		terminals: map[string][]ghostty.TerminalRef{},
	}
}

func (f *fakeGhosttyClient) Available() error     { return nil }
func (f *fakeGhosttyClient) EnsureRunning() error { return nil }
func (f *fakeGhosttyClient) FocusTerminal(string) error {
	return nil
}
func (f *fakeGhosttyClient) InputText(terminalID string, text string) error {
	f.mu.Lock()
	f.inputTexts = append(f.inputTexts, fakeGhosttyInput{terminalID: terminalID, text: text})
	hook := f.inputTextHook
	f.mu.Unlock()
	if hook != nil {
		return hook(terminalID, text)
	}
	return nil
}
func (f *fakeGhosttyClient) SendKey(terminalID string, key string, modifiers []string) error {
	f.mu.Lock()
	f.sendKeyCalls = append(f.sendKeyCalls, fakeGhosttyKey{
		terminalID: terminalID,
		key:        key,
		modifiers:  append([]string(nil), modifiers...),
	})
	hook := f.sendKeyHook
	f.mu.Unlock()
	if hook != nil {
		return hook(terminalID, key, modifiers)
	}
	return nil
}
func (f *fakeGhosttyClient) InspectFocused() (ghostty.FocusContext, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.focus.Terminal.ID == "" {
		return ghostty.FocusContext{}, errors.New("no focused terminal")
	}
	return f.focus, nil
}

func (f *fakeGhosttyClient) NewWindow(string) (ghostty.WindowRef, ghostty.TerminalRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.windowCounter++
	f.tabCounter++
	f.terminalCounter++
	windowID := "window-test-" + itoa(f.windowCounter)
	tabID := "tab-test-" + itoa(f.tabCounter)
	terminalID := "term-test-" + itoa(f.terminalCounter)
	window := ghostty.WindowRef{ID: windowID, Name: windowID, SelectedTabID: tabID}
	tab := ghostty.TabRef{ID: tabID, Name: tabID, Index: 1, Selected: true, FocusedTerminalID: terminalID}
	terminal := ghostty.TerminalRef{ID: terminalID, Name: terminalID}
	f.windows[windowID] = window
	f.tabs[windowID] = []ghostty.TabRef{tab}
	f.terminals[tabID] = []ghostty.TerminalRef{terminal}
	f.focus = ghostty.FocusContext{Window: window, Tab: tab, Terminal: terminal}
	return window, terminal, nil
}

func (f *fakeGhosttyClient) NewTab(windowID string, _ string) (ghostty.TabRef, ghostty.TerminalRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tabCounter++
	f.terminalCounter++
	tabID := "tab-test-" + itoa(f.tabCounter)
	terminalID := "term-test-" + itoa(f.terminalCounter)
	tab := ghostty.TabRef{ID: tabID, Name: tabID, Index: len(f.tabs[windowID]) + 1, Selected: true, FocusedTerminalID: terminalID}
	terminal := ghostty.TerminalRef{ID: terminalID, Name: terminalID}
	f.tabs[windowID] = append(f.tabs[windowID], tab)
	f.terminals[tabID] = []ghostty.TerminalRef{terminal}
	window := f.windows[windowID]
	window.SelectedTabID = tabID
	f.windows[windowID] = window
	f.focus = ghostty.FocusContext{Window: window, Tab: tab, Terminal: terminal}
	return tab, terminal, nil
}

func (f *fakeGhosttyClient) SplitTerminal(terminalID string, _ string, _ string) (ghostty.TerminalRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.splitErr != nil {
		return ghostty.TerminalRef{}, f.splitErr
	}
	f.terminalCounter++
	newTerminal := ghostty.TerminalRef{ID: "term-test-" + itoa(f.terminalCounter), Name: terminalID}
	for tabID, terminals := range f.terminals {
		for _, existing := range terminals {
			if existing.ID == terminalID {
				f.terminals[tabID] = append(f.terminals[tabID], newTerminal)
				for windowID, tabs := range f.tabs {
					for _, tab := range tabs {
						if tab.ID == tabID {
							window := f.windows[windowID]
							updatedTab := tab
							updatedTab.FocusedTerminalID = newTerminal.ID
							f.focus = ghostty.FocusContext{Window: window, Tab: updatedTab, Terminal: newTerminal}
							return newTerminal, nil
						}
					}
				}
				return newTerminal, nil
			}
		}
	}
	return newTerminal, nil
}

func (f *fakeGhosttyClient) ListWindows() ([]ghostty.WindowRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ghostty.WindowRef, 0, len(f.windows))
	for _, window := range f.windows {
		out = append(out, window)
	}
	return out, nil
}

func (f *fakeGhosttyClient) ListTabs(windowID string) ([]ghostty.TabRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ghostty.TabRef(nil), f.tabs[windowID]...), nil
}

func (f *fakeGhosttyClient) ListTerminals(tabID string) ([]ghostty.TerminalRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ghostty.TerminalRef(nil), f.terminals[tabID]...), nil
}

func (f fakeJumpClient) SearchHost(query string) ([]jump.HostMatch, error) {
	return []jump.HostMatch{{DisplayName: query}}, nil
}

func (f fakeJumpClient) AttachHost(localTarget string, hostQuery string) (jump.ResolvedHost, error) {
	return jump.ResolvedHost{Query: hostQuery, Name: hostQuery, RemoteSession: "tmux-ghostty"}, nil
}

func (f fakeJumpClient) EnsureRemoteTmux(localTarget string, remoteSession string) error { return nil }
func (f fakeJumpClient) Reconnect(localTarget string) error                              { return nil }

func TestShouldAutoExitLocked(t *testing.T) {
	service := newTestService(t)
	now := time.Now().UTC()

	service.state.LastRequestAt = now.Add(-service.idleTimeout).Add(-time.Second)
	if !service.shouldAutoExitLocked(now) {
		t.Fatalf("expected auto exit when idle with no active workspace or pane")
	}

	workspace := model.NewWorkspace()
	pane := model.NewPane(workspace.ID)
	workspace.PaneIDs = []string{pane.ID}
	service.state.Workspaces[workspace.ID] = workspace
	service.state.Panes[pane.ID] = pane
	if service.shouldAutoExitLocked(now) {
		t.Fatalf("did not expect auto exit with active workspace and pane")
	}
}

func TestCommandFlowWithTmux(t *testing.T) {
	service := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Start(ctx)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}

	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	pane := created.Pane
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})

	if _, err := service.Claim(pane.ID, "agent"); err != nil {
		t.Fatalf("claim agent: %v", err)
	}
	if _, err := service.SendCommand(pane.ID, "agent", "pwd", ""); err != nil {
		t.Fatalf("send pwd: %v", err)
	}
	if _, err := waitForSnapshot(t, service, pane.ID, cwd); err != nil {
		t.Fatalf("wait for pwd output: %v", err)
	}

	targetFile := filepath.Join(t.TempDir(), "approval-flow.txt")
	preview, err := service.PreviewCommand(pane.ID, "agent", "echo risky > "+targetFile)
	if err != nil {
		t.Fatalf("preview risky command: %v", err)
	}
	if !preview.RequiresApproval || preview.Action == nil {
		t.Fatalf("expected approval to be required")
	}
	if _, err := service.Approve(preview.Action.ID); err != nil {
		t.Fatalf("approve action: %v", err)
	}
	if err := waitForFile(targetFile, 5*time.Second); err != nil {
		t.Fatalf("wait for risky command side effect: %v", err)
	}

	sleepPreview, err := service.PreviewCommand(pane.ID, "agent", "sleep 30")
	if err != nil {
		t.Fatalf("preview sleep command: %v", err)
	}
	if _, err := service.Approve(sleepPreview.Action.ID); err != nil {
		t.Fatalf("approve sleep action: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	interrupted, err := service.InterruptPane(pane.ID)
	if err != nil {
		t.Fatalf("interrupt pane: %v", err)
	}
	if interrupted.Mode != model.ModeIdle {
		t.Fatalf("expected pane to become idle after interrupt, got %q", interrupted.Mode)
	}

	released, err := service.Release(pane.ID)
	if err != nil {
		t.Fatalf("release pane: %v", err)
	}
	if released.Controller != model.ControllerUser {
		t.Fatalf("expected controller user after release, got %q", released.Controller)
	}
}

func TestInspectAndAdoptCurrent(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	window, terminal, err := fakeGhostty.NewWindow("")
	if err != nil {
		t.Fatalf("seed focused window: %v", err)
	}
	sessionName := fmt.Sprintf("adopt-%d", time.Now().UnixNano())
	if err := service.tmux.NewSession(sessionName); err != nil {
		t.Fatalf("create adopt tmux session: %v", err)
	}
	t.Cleanup(func() {
		_ = service.tmux.KillSession(sessionName)
	})
	service.probeCurrent = func(terminalID string) (currentTerminalProbe, error) {
		if terminalID != terminal.ID {
			t.Fatalf("unexpected terminal id: %s", terminalID)
		}
		return currentTerminalProbe{
			InsideTmux:  true,
			TmuxSession: sessionName,
			TmuxPane:    sessionName + ":0.0",
		}, nil
	}

	inspection, err := service.InspectCurrent()
	if err != nil {
		t.Fatalf("inspect current: %v", err)
	}
	if !inspection.Adoptable {
		t.Fatalf("expected current focus to be adoptable, got %+v", inspection)
	}

	result, err := service.AdoptCurrent()
	if err != nil {
		t.Fatalf("adopt current: %v", err)
	}
	if result.Workspace.GhosttyWindowID != window.ID {
		t.Fatalf("expected adopted workspace to stay in focused window")
	}
	if result.Pane.GhosttyTerminalID != terminal.ID {
		t.Fatalf("expected adopted pane to reuse focused terminal")
	}
	if result.Pane.LocalTmuxSession != sessionName {
		t.Fatalf("unexpected adopted local tmux session: %q", result.Pane.LocalTmuxSession)
	}
	if result.Pane.LocalTmuxTarget != sessionName+":0.0" {
		t.Fatalf("unexpected adopted local tmux target: %q", result.Pane.LocalTmuxTarget)
	}
	if result.Pane.OwnsLocalTmux {
		t.Fatalf("adopted pane must not own the pre-existing tmux session")
	}
}

func TestAdoptCurrentFailsWhenNotInsideTmux(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	if _, _, err := fakeGhostty.NewWindow(""); err != nil {
		t.Fatalf("seed focused window: %v", err)
	}
	service.probeCurrent = func(string) (currentTerminalProbe, error) {
		return currentTerminalProbe{InsideTmux: false}, nil
	}

	inspection, err := service.InspectCurrent()
	if err != nil {
		t.Fatalf("inspect current: %v", err)
	}
	if inspection.Adoptable {
		t.Fatalf("expected current focus to be non-adoptable")
	}
	if _, err := service.AdoptCurrent(); err == nil {
		t.Fatalf("expected adopt current to fail outside tmux")
	}
}

func TestAdoptCurrentFailsWhenCurrentTerminalAlreadyManaged(t *testing.T) {
	service := newTestService(t)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})

	inspection, err := service.InspectCurrent()
	if err != nil {
		t.Fatalf("inspect current: %v", err)
	}
	if !inspection.Managed {
		t.Fatalf("expected focused terminal to already be managed")
	}
	if _, err := service.AdoptCurrent(); err == nil {
		t.Fatalf("expected adopt current to fail for a managed terminal")
	}
}

func TestProbeCurrentTerminalUsesShortTempScript(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)

	var typedCommand string
	fakeGhostty.inputTextHook = func(_ string, text string) error {
		typedCommand = text
		return nil
	}
	fakeGhostty.sendKeyHook = func(_ string, key string, _ []string) error {
		if key != "enter" {
			return fmt.Errorf("unexpected key: %s", key)
		}
		scriptPath := strings.TrimSpace(typedCommand)
		script, err := os.ReadFile(scriptPath)
		if err != nil {
			return fmt.Errorf("read probe script: %w", err)
		}
		if !strings.Contains(string(script), "tmux display-message") {
			return fmt.Errorf("probe script did not include tmux probe")
		}
		probePath := strings.TrimSuffix(scriptPath, ".sh") + ".json"
		return os.WriteFile(probePath, []byte(`{"inside_tmux":true,"tmux_session":"local-test","tmux_pane":"local-test:0.0"}`), 0o600)
	}

	probe, err := service.probeCurrentTerminal("terminal-test")
	if err != nil {
		t.Fatalf("probe current terminal: %v", err)
	}
	if !probe.InsideTmux {
		t.Fatalf("expected probe to report tmux")
	}
	if probe.TmuxSession != "local-test" {
		t.Fatalf("unexpected tmux session: %q", probe.TmuxSession)
	}
	if probe.TmuxPane != "local-test:0.0" {
		t.Fatalf("unexpected tmux pane: %q", probe.TmuxPane)
	}
	if !strings.HasPrefix(typedCommand, " /tmp/tmux-ghostty-probe-") {
		t.Fatalf("expected short temp script command, got %q", typedCommand)
	}
	if strings.Contains(typedCommand, "tmux display-message") || strings.Contains(typedCommand, "inside_tmux") {
		t.Fatalf("expected injected command to hide inline probe shell, got %q", typedCommand)
	}

	scriptPath := strings.TrimSpace(typedCommand)
	if _, err := os.Stat(scriptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected probe script to be cleaned up, stat err=%v", err)
	}
	probePath := strings.TrimSuffix(scriptPath, ".sh") + ".json"
	if _, err := os.Stat(probePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected probe json to be cleaned up, stat err=%v", err)
	}
}

func TestSplitPaneAddsPaneToWorkspace(t *testing.T) {
	service := newTestService(t)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})

	newPane, err := service.SplitPane(created.Pane.ID, "up", "agent")
	if err != nil {
		t.Fatalf("split pane: %v", err)
	}
	if newPane.WorkspaceID != created.Workspace.ID {
		t.Fatalf("expected split pane to stay in the same workspace")
	}
	if newPane.Controller != model.ControllerAgent {
		t.Fatalf("expected split pane claim to set controller agent, got %q", newPane.Controller)
	}
	if !newPane.OwnsLocalTmux {
		t.Fatalf("expected broker-created split pane to own its local tmux session")
	}
	if len(service.state.Workspaces[created.Workspace.ID].PaneIDs) != 2 {
		t.Fatalf("expected workspace to contain two panes after split")
	}
}

func TestSplitPaneRollsBackStateOnGhosttyFailure(t *testing.T) {
	service := newTestService(t)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})

	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	fakeGhostty.splitErr = errors.New("split failed")
	if _, err := service.SplitPane(created.Pane.ID, "right", ""); err == nil {
		t.Fatalf("expected split pane to fail")
	}
	if len(service.state.Workspaces[created.Workspace.ID].PaneIDs) != 1 {
		t.Fatalf("expected workspace pane membership to roll back on split failure")
	}
	if len(service.state.Panes) != 1 {
		t.Fatalf("expected pane state to roll back on split failure")
	}
}

func TestReconcileDoesNotImportUnmanagedCurrentWindow(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	if _, _, err := fakeGhostty.NewWindow(""); err != nil {
		t.Fatalf("seed unmanaged window: %v", err)
	}

	workspaces, err := service.Reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(workspaces) != 0 {
		t.Fatalf("expected reconcile to ignore unmanaged Ghostty windows, got %d workspaces", len(workspaces))
	}
	if len(service.state.Workspaces) != 0 || len(service.state.Panes) != 0 {
		t.Fatalf("expected no imported state after reconcile")
	}
}

func TestRPCServerRoundTrip(t *testing.T) {
	service := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.SetShutdownFunc(cancel)
	service.Start(ctx)

	socketPath := filepath.Join(t.TempDir(), "broker.sock")
	server := rpc.Server{
		SocketPath: socketPath,
		Handler:    service.HandleRPC,
	}
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Listen(ctx)
	}()
	waitForSocket(t, socketPath)

	client := rpc.NewClient(socketPath)
	var created WorkspaceCreateResult
	if err := client.Call(ctx, "workspace.create", nil, &created); err != nil {
		t.Fatalf("rpc workspace.create: %v", err)
	}

	var panes []model.Pane
	if err := client.Call(ctx, "pane.list", nil, &panes); err != nil {
		t.Fatalf("rpc pane.list: %v", err)
	}
	if len(panes) != 1 {
		t.Fatalf("expected one pane after workspace.create, got %d", len(panes))
	}

	if err := client.Call(ctx, "broker.shutdown", map[string]any{"force": true}, &struct{}{}); err != nil {
		t.Fatalf("rpc broker.shutdown: %v", err)
	}
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("server exit: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for rpc server to stop")
	}
	_ = service.CloseWorkspace(created.Workspace.ID)
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	logger, err := logx.New("")
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	runner := execx.NewRunner(logger)
	tmuxClient := tmux.New(runner)
	service, err := NewService(
		filepath.Join(dir, "state.json"),
		filepath.Join(dir, "actions.json"),
		2*time.Second,
		logger,
		newFakeGhosttyClient(),
		tmuxClient,
		fakeJumpClient{},
	)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return service
}

func waitForSnapshot(t *testing.T, service *Service, paneID string, substring string) (model.PaneSnapshot, error) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, err := service.SnapshotPane(paneID)
		if err != nil {
			return model.PaneSnapshot{}, err
		}
		if strings.Contains(snapshot.Text, substring) {
			return snapshot, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return model.PaneSnapshot{}, context.DeadlineExceeded
}

func waitForFile(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return context.DeadlineExceeded
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("socket did not appear: %s", path)
}

func itoa(value int) string {
	return fmt.Sprintf("%d", value)
}
