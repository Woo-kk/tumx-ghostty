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
	"github.com/Woo-kk/tmux-ghostty/internal/logx"
	"github.com/Woo-kk/tmux-ghostty/internal/model"
	"github.com/Woo-kk/tmux-ghostty/internal/remote"
	"github.com/Woo-kk/tmux-ghostty/internal/rpc"
	"github.com/Woo-kk/tmux-ghostty/internal/tmux"
)

type fakeGhosttyClient struct {
	mu              sync.Mutex
	windowCounter   int
	tabCounter      int
	terminalCounter int
	splitErr        error
	requireErr      error
	requireCalls    int
	ensureErr       error
	ensureCalls     int
	newWindowCalls  int
	inputTextHook   func(string, string) error
	sendKeyHook     func(string, string, []string) error
	inspectHook     func() (ghostty.FocusContext, error)
	inputTexts      []fakeGhosttyInput
	sendKeyCalls    []fakeGhosttyKey
	inspectResults  []fakeGhosttyFocusResult
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

type fakeGhosttyFocusResult struct {
	focus ghostty.FocusContext
	err   error
}

type fakeRemoteClient struct {
	attachResult remote.ResolvedTarget
	attachErr    error
}

func newFakeGhosttyClient() *fakeGhosttyClient {
	return &fakeGhosttyClient{
		windows:   map[string]ghostty.WindowRef{},
		tabs:      map[string][]ghostty.TabRef{},
		terminals: map[string][]ghostty.TerminalRef{},
	}
}

func (f *fakeGhosttyClient) RequireAvailable() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requireCalls++
	return f.requireErr
}

func (f *fakeGhosttyClient) Available() error {
	return f.RequireAvailable()
}

func (f *fakeGhosttyClient) EnsureRunning() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensureCalls++
	return f.ensureErr
}

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
	if len(f.inspectResults) > 0 {
		result := f.inspectResults[0]
		if len(f.inspectResults) > 1 {
			f.inspectResults = f.inspectResults[1:]
		}
		f.mu.Unlock()
		return result.focus, result.err
	}
	hook := f.inspectHook
	focus := f.focus
	f.mu.Unlock()
	if hook != nil {
		return hook()
	}
	if focus.Terminal.ID == "" {
		return ghostty.FocusContext{}, errors.New("no focused terminal")
	}
	return focus, nil
}

func (f *fakeGhosttyClient) NewWindow(string) (ghostty.WindowRef, ghostty.TerminalRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.newWindowCalls++
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

func (f fakeRemoteClient) SearchTarget(query string) ([]remote.TargetMatch, error) {
	return []remote.TargetMatch{{DisplayName: query}}, nil
}

func (f fakeRemoteClient) AttachTarget(localTarget string, query string) (remote.ResolvedTarget, error) {
	if f.attachErr != nil {
		return remote.ResolvedTarget{}, f.attachErr
	}
	result := f.attachResult
	if strings.TrimSpace(result.Query) == "" {
		result.Query = query
	}
	if strings.TrimSpace(result.Name) == "" {
		result.Name = query
	}
	if strings.TrimSpace(result.RemoteSession) == "" {
		result.RemoteSession = "tmux-ghostty"
	}
	if strings.TrimSpace(result.Provider) == "" {
		result.Provider = remote.ProviderJumpServer
	}
	if result.RemoteTmuxStatus == "" {
		result.RemoteTmuxStatus = model.RemoteTmuxStatusAttached
	}
	return result, nil
}

func (f fakeRemoteClient) EnsureRemoteSession(localTarget string, remoteSession string) error {
	return nil
}

func (f fakeRemoteClient) Reconnect(localTarget string) error { return nil }

func (f fakeRemoteClient) DetectStage(text string) model.PaneStage {
	return remote.DetectStage(text)
}

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

func TestCreateWorkspaceOpensNewGhosttyWindow(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	fakeGhostty.newWindowCalls = 0

	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})

	if created.Workspace.LaunchMode != model.WorkspaceLaunchModeNewWindow {
		t.Fatalf("expected create workspace launch mode new_window, got %q", created.Workspace.LaunchMode)
	}
	if strings.TrimSpace(created.Workspace.GhosttyWindowID) == "" {
		t.Fatalf("expected create workspace to record a Ghostty window id")
	}
	if strings.TrimSpace(created.Pane.GhosttyTerminalID) == "" {
		t.Fatalf("expected create workspace to record a Ghostty terminal id")
	}
	if fakeGhostty.newWindowCalls != 1 {
		t.Fatalf("expected create workspace to create exactly one Ghostty window, got %d", fakeGhostty.newWindowCalls)
	}
}

func TestListWorkspaceWindowsIncludesManagedMetadata(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)

	unmanagedWindow, unmanagedTerminal, err := fakeGhostty.NewWindow("")
	if err != nil {
		t.Fatalf("seed unmanaged window: %v", err)
	}
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})
	fakeGhostty.ensureCalls = 0
	fakeGhostty.newWindowCalls = 0

	targets, err := service.ListWorkspaceWindows()
	if err != nil {
		t.Fatalf("list workspace windows: %v", err)
	}

	var unmanagedTarget *model.WorkspaceTerminalTarget
	var managedTarget *model.WorkspaceTerminalTarget
	for index := range targets {
		target := &targets[index]
		switch target.TerminalID {
		case unmanagedTerminal.ID:
			unmanagedTarget = target
		case created.Pane.GhosttyTerminalID:
			managedTarget = target
		}
	}

	if unmanagedTarget == nil {
		t.Fatalf("expected unmanaged terminal %s in list result: %+v", unmanagedTerminal.ID, targets)
	}
	if managedTarget == nil {
		t.Fatalf("expected managed terminal %s in list result: %+v", created.Pane.GhosttyTerminalID, targets)
	}
	if unmanagedTarget.Managed {
		t.Fatalf("expected unmanaged terminal metadata, got %+v", unmanagedTarget)
	}
	if unmanagedTarget.WindowID != unmanagedWindow.ID {
		t.Fatalf("expected unmanaged target to stay in window %s, got %+v", unmanagedWindow.ID, unmanagedTarget)
	}
	if !managedTarget.Managed {
		t.Fatalf("expected managed terminal metadata, got %+v", managedTarget)
	}
	if managedTarget.ManagedPaneID != created.Pane.ID {
		t.Fatalf("expected managed pane id %s, got %+v", created.Pane.ID, managedTarget)
	}
	if managedTarget.ManagedWorkspaceID != created.Workspace.ID {
		t.Fatalf("expected managed workspace id %s, got %+v", created.Workspace.ID, managedTarget)
	}
	if fakeGhostty.ensureCalls != 0 {
		t.Fatalf("expected list workspace windows not to call EnsureRunning, got %d", fakeGhostty.ensureCalls)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected list workspace windows not to create a new Ghostty window, got %d", fakeGhostty.newWindowCalls)
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

func TestAttachHostPersistsRemoteTmuxMetadata(t *testing.T) {
	service := newTestServiceWithRemote(t, fakeRemoteClient{
		attachResult: remote.ResolvedTarget{
			RemoteSession:    "tmux-ghostty",
			Provider:         remote.ProviderJumpServer,
			RemoteTmuxStatus: model.RemoteTmuxStatusUnavailable,
			RemoteTmuxDetail: "tmux not found",
		},
	})
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})

	result, err := service.AttachHost(created.Pane.ID, "2801")
	if err != nil {
		t.Fatalf("attach host: %v", err)
	}
	if result.Pane.RemoteTmuxStatus != model.RemoteTmuxStatusUnavailable {
		t.Fatalf("expected pane remote tmux status unavailable, got %q", result.Pane.RemoteTmuxStatus)
	}
	if result.Pane.RemoteTmuxDetail != "tmux not found" {
		t.Fatalf("unexpected pane remote tmux detail: %q", result.Pane.RemoteTmuxDetail)
	}
	if result.Target.RemoteTmuxStatus != model.RemoteTmuxStatusUnavailable {
		t.Fatalf("expected target remote tmux status unavailable, got %q", result.Target.RemoteTmuxStatus)
	}

	snapshot, err := service.SnapshotPane(created.Pane.ID)
	if err != nil {
		t.Fatalf("snapshot pane: %v", err)
	}
	if snapshot.RemoteTmuxStatus != model.RemoteTmuxStatusUnavailable {
		t.Fatalf("expected snapshot remote tmux status unavailable, got %q", snapshot.RemoteTmuxStatus)
	}
	if snapshot.RemoteTmuxDetail != "tmux not found" {
		t.Fatalf("unexpected snapshot remote tmux detail: %q", snapshot.RemoteTmuxDetail)
	}
}

func TestInspectCurrentUsesRequireAvailableWithoutLaunch(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	_, terminal, err := fakeGhostty.NewWindow("")
	if err != nil {
		t.Fatalf("seed focused window: %v", err)
	}
	fakeGhostty.newWindowCalls = 0

	sessionName := fmt.Sprintf("inspect-%d", time.Now().UnixNano())
	if err := service.tmux.NewSession(sessionName); err != nil {
		t.Fatalf("create inspect tmux session: %v", err)
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
	if fakeGhostty.requireCalls != 1 {
		t.Fatalf("expected RequireAvailable to be used once, got %d", fakeGhostty.requireCalls)
	}
	if fakeGhostty.ensureCalls != 0 {
		t.Fatalf("expected current-window inspect not to call EnsureRunning, got %d", fakeGhostty.ensureCalls)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected inspect current not to create a new window, got %d", fakeGhostty.newWindowCalls)
	}
}

func TestAdoptCurrentUsesRequireAvailableWithoutLaunch(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	window, terminal, err := fakeGhostty.NewWindow("")
	if err != nil {
		t.Fatalf("seed focused window: %v", err)
	}
	fakeGhostty.newWindowCalls = 0

	sessionName := fmt.Sprintf("adopt-no-launch-%d", time.Now().UnixNano())
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

	result, err := service.AdoptCurrent()
	if err != nil {
		t.Fatalf("adopt current: %v", err)
	}
	if result.Workspace.GhosttyWindowID != window.ID {
		t.Fatalf("expected adopted workspace to stay in focused window")
	}
	if fakeGhostty.requireCalls != 1 {
		t.Fatalf("expected RequireAvailable to be used once, got %d", fakeGhostty.requireCalls)
	}
	if fakeGhostty.ensureCalls != 0 {
		t.Fatalf("expected adopt current not to call EnsureRunning, got %d", fakeGhostty.ensureCalls)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected adopt current not to create a new window, got %d", fakeGhostty.newWindowCalls)
	}
}

func TestInspectCurrentReportsBootstrappableLocalShell(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	if _, terminal, err := fakeGhostty.NewWindow(""); err != nil {
		t.Fatalf("seed focused window: %v", err)
	} else {
		service.probeCurrent = func(terminalID string) (currentTerminalProbe, error) {
			if terminalID != terminal.ID {
				t.Fatalf("unexpected terminal id: %s", terminalID)
			}
			return currentTerminalProbe{
				InsideTmux:    false,
				TmuxAvailable: true,
			}, nil
		}
	}
	fakeGhostty.newWindowCalls = 0

	inspection, err := service.InspectCurrent()
	if err != nil {
		t.Fatalf("inspect current: %v", err)
	}
	if inspection.Adoptable {
		t.Fatalf("expected current focus not to be directly adoptable")
	}
	if !inspection.Bootstrappable {
		t.Fatalf("expected current focus to be bootstrappable")
	}
	if !strings.Contains(inspection.Reason, "workspace bootstrap-current") {
		t.Fatalf("unexpected inspect current reason: %q", inspection.Reason)
	}
	if fakeGhostty.ensureCalls != 0 {
		t.Fatalf("expected inspect current not to call EnsureRunning, got %d", fakeGhostty.ensureCalls)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected inspect current not to create a new window, got %d", fakeGhostty.newWindowCalls)
	}
}

func TestBootstrapCurrentUsesCurrentWindowWithoutLaunch(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	window, terminal, err := fakeGhostty.NewWindow("")
	if err != nil {
		t.Fatalf("seed focused window: %v", err)
	}
	fakeGhostty.newWindowCalls = 0

	var attachCommand string
	var bootstrappedSession string
	fakeGhostty.inputTextHook = func(terminalID string, text string) error {
		if terminalID != terminal.ID {
			t.Fatalf("unexpected terminal id: %s", terminalID)
		}
		attachCommand = text
		parts := strings.Split(text, "-t ")
		if len(parts) != 2 {
			t.Fatalf("unexpected bootstrap attach command: %q", text)
		}
		bootstrappedSession = strings.TrimSpace(parts[1])
		return nil
	}

	probeCalls := 0
	service.probeCurrent = func(terminalID string) (currentTerminalProbe, error) {
		if terminalID != terminal.ID {
			t.Fatalf("unexpected terminal id: %s", terminalID)
		}
		probeCalls++
		if probeCalls == 1 {
			return currentTerminalProbe{
				InsideTmux:    false,
				TmuxAvailable: true,
			}, nil
		}
		if bootstrappedSession == "" {
			t.Fatalf("expected bootstrap session to be captured before second probe")
		}
		return currentTerminalProbe{
			InsideTmux:    true,
			TmuxSession:   bootstrappedSession,
			TmuxPane:      bootstrappedSession + ":0.0",
			TmuxAvailable: true,
		}, nil
	}

	result, err := service.BootstrapCurrent()
	if err != nil {
		t.Fatalf("bootstrap current: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(result.Workspace.ID)
	})

	if !strings.Contains(attachCommand, "exec tmux attach-session -t ") {
		t.Fatalf("unexpected bootstrap command: %q", attachCommand)
	}
	if result.Workspace.GhosttyWindowID != window.ID {
		t.Fatalf("expected bootstrapped workspace to stay in focused window")
	}
	if result.Workspace.LaunchMode != model.WorkspaceLaunchModeCurrentWindow {
		t.Fatalf("expected bootstrapped workspace launch mode current_window, got %q", result.Workspace.LaunchMode)
	}
	if result.Pane.GhosttyTerminalID != terminal.ID {
		t.Fatalf("expected bootstrapped pane to reuse focused terminal")
	}
	if result.Pane.LocalTmuxSession != bootstrappedSession {
		t.Fatalf("unexpected bootstrapped local tmux session: %q", result.Pane.LocalTmuxSession)
	}
	if !result.Pane.OwnsLocalTmux {
		t.Fatalf("expected bootstrapped pane to own its local tmux session")
	}
	if fakeGhostty.requireCalls == 0 {
		t.Fatalf("expected bootstrap current to probe Ghostty availability")
	}
	if fakeGhostty.ensureCalls != 0 {
		t.Fatalf("expected bootstrap current not to call EnsureRunning, got %d", fakeGhostty.ensureCalls)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected bootstrap current not to create a new window, got %d", fakeGhostty.newWindowCalls)
	}
}

func TestBootstrapCurrentFailsForRemoteShell(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	if _, terminal, err := fakeGhostty.NewWindow(""); err != nil {
		t.Fatalf("seed focused window: %v", err)
	} else {
		service.probeCurrent = func(terminalID string) (currentTerminalProbe, error) {
			if terminalID != terminal.ID {
				t.Fatalf("unexpected terminal id: %s", terminalID)
			}
			return currentTerminalProbe{
				InsideTmux:    false,
				RemoteShell:   true,
				TmuxAvailable: true,
			}, nil
		}
	}

	if _, err := service.BootstrapCurrent(); err == nil {
		t.Fatalf("expected bootstrap current to fail for remote shell")
	}
	if len(service.state.Workspaces) != 0 {
		t.Fatalf("expected no workspace to be created for remote shell bootstrap attempt")
	}
}

func TestSplitCurrentUsesFocusedWindowWithoutLaunch(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	window, terminal, err := fakeGhostty.NewWindow("")
	if err != nil {
		t.Fatalf("seed focused window: %v", err)
	}
	fakeGhostty.newWindowCalls = 0

	result, err := service.SplitCurrent("up", "agent")
	if err != nil {
		t.Fatalf("split current: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(result.Workspace.ID)
	})

	if result.Workspace.GhosttyWindowID != window.ID {
		t.Fatalf("expected split-current workspace to stay in focused window")
	}
	if result.Workspace.LaunchMode != model.WorkspaceLaunchModeCurrentWindow {
		t.Fatalf("expected split-current workspace launch mode current_window, got %q", result.Workspace.LaunchMode)
	}
	if result.Pane.Controller != model.ControllerAgent {
		t.Fatalf("expected claimed pane controller agent, got %q", result.Pane.Controller)
	}
	if result.Pane.GhosttyTerminalID == terminal.ID {
		t.Fatalf("expected split-current to create a new terminal instead of reusing the focused terminal")
	}
	if !result.Pane.OwnsLocalTmux {
		t.Fatalf("expected split-current pane to own its local tmux session")
	}
	if fakeGhostty.requireCalls == 0 {
		t.Fatalf("expected split-current to probe Ghostty availability")
	}
	if fakeGhostty.ensureCalls != 0 {
		t.Fatalf("expected split-current not to call EnsureRunning, got %d", fakeGhostty.ensureCalls)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected split-current not to create a new window, got %d", fakeGhostty.newWindowCalls)
	}
}

func TestSplitCurrentManagedFocusSuggestsExplicitTargeting(t *testing.T) {
	service := newTestService(t)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})

	_, err = service.SplitCurrent("up", "")
	if err == nil {
		t.Fatalf("expected split-current to fail for a managed focused terminal")
	}
	if !strings.Contains(err.Error(), "workspace split-current only targets the focused Ghostty terminal") {
		t.Fatalf("expected split-current guidance in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "workspace list-windows") || !strings.Contains(err.Error(), "workspace split-terminal") {
		t.Fatalf("expected explicit-target guidance in error, got %v", err)
	}
}

func TestSplitTerminalUsesExplicitTargetWithoutFocus(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)

	targetWindow, targetTerminal, err := fakeGhostty.NewWindow("")
	if err != nil {
		t.Fatalf("seed target window: %v", err)
	}
	if _, _, err := fakeGhostty.NewWindow(""); err != nil {
		t.Fatalf("seed focused window: %v", err)
	}
	fakeGhostty.newWindowCalls = 0

	result, err := service.SplitTerminal(targetTerminal.ID, "up", "agent")
	if err != nil {
		t.Fatalf("split terminal: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(result.Workspace.ID)
	})

	if result.Workspace.GhosttyWindowID != targetWindow.ID {
		t.Fatalf("expected split-terminal workspace to stay in target window %s, got %+v", targetWindow.ID, result.Workspace)
	}
	if result.Workspace.LaunchMode != model.WorkspaceLaunchModeCurrentWindow {
		t.Fatalf("expected split-terminal launch mode current_window, got %q", result.Workspace.LaunchMode)
	}
	if result.Pane.Controller != model.ControllerAgent {
		t.Fatalf("expected claimed pane controller agent, got %q", result.Pane.Controller)
	}
	if result.Pane.GhosttyTerminalID == targetTerminal.ID {
		t.Fatalf("expected split-terminal to create a new terminal instead of reusing the target terminal")
	}
	if !result.Pane.OwnsLocalTmux {
		t.Fatalf("expected split-terminal pane to own its local tmux session")
	}
	if fakeGhostty.requireCalls == 0 {
		t.Fatalf("expected split-terminal to probe Ghostty availability")
	}
	if fakeGhostty.ensureCalls != 0 {
		t.Fatalf("expected split-terminal not to call EnsureRunning, got %d", fakeGhostty.ensureCalls)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected split-terminal not to create a new Ghostty window, got %d", fakeGhostty.newWindowCalls)
	}
}

func TestSplitTerminalRejectsUnknownTerminal(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	if _, _, err := fakeGhostty.NewWindow(""); err != nil {
		t.Fatalf("seed Ghostty window: %v", err)
	}
	fakeGhostty.newWindowCalls = 0

	_, err := service.SplitTerminal("missing-terminal", "up", "")
	if err == nil {
		t.Fatalf("expected split-terminal to fail for an unknown terminal")
	}
	var brokerErr *BrokerError
	if !errors.As(err, &brokerErr) || brokerErr.Reason != rpc.ReasonInvalidState {
		t.Fatalf("expected invalid_state broker error, got %v", err)
	}
	if !strings.Contains(err.Error(), "ghostty terminal not found: missing-terminal") {
		t.Fatalf("unexpected split-terminal error: %v", err)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected split-terminal unknown target not to create a new Ghostty window, got %d", fakeGhostty.newWindowCalls)
	}
}

func TestSplitTerminalRejectsManagedTerminal(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})
	fakeGhostty.newWindowCalls = 0

	_, err = service.SplitTerminal(created.Pane.GhosttyTerminalID, "up", "")
	if err == nil {
		t.Fatalf("expected split-terminal to fail for a managed terminal")
	}
	if !strings.Contains(err.Error(), created.Pane.ID) || !strings.Contains(err.Error(), "use pane split instead") {
		t.Fatalf("expected pane split guidance in error, got %v", err)
	}
	if len(service.state.Workspaces) != 1 || len(service.state.Panes) != 1 {
		t.Fatalf("expected managed-target split-terminal failure to leave state unchanged")
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected split-terminal managed target not to create a new Ghostty window, got %d", fakeGhostty.newWindowCalls)
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

func TestCurrentWindowFailsWhenFocusChangesDuringProbe(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)
	firstFocus := ghostty.FocusContext{
		Window:   ghostty.WindowRef{ID: "window-a", Name: "window-a", SelectedTabID: "tab-a"},
		Tab:      ghostty.TabRef{ID: "tab-a", Name: "tab-a", Index: 1, Selected: true, FocusedTerminalID: "term-a"},
		Terminal: ghostty.TerminalRef{ID: "term-a", Name: "term-a", WorkingDirectory: "/tmp/a"},
	}
	secondFocus := ghostty.FocusContext{
		Window:   ghostty.WindowRef{ID: "window-b", Name: "window-b", SelectedTabID: "tab-b"},
		Tab:      ghostty.TabRef{ID: "tab-b", Name: "tab-b", Index: 1, Selected: true, FocusedTerminalID: "term-b"},
		Terminal: ghostty.TerminalRef{ID: "term-b", Name: "term-b", WorkingDirectory: "/tmp/b"},
	}
	fakeGhostty.focus = firstFocus
	fakeGhostty.inspectResults = []fakeGhosttyFocusResult{
		{focus: firstFocus},
		{focus: secondFocus},
		{focus: firstFocus},
		{focus: secondFocus},
	}
	service.probeCurrent = func(terminalID string) (currentTerminalProbe, error) {
		if terminalID != firstFocus.Terminal.ID {
			t.Fatalf("unexpected terminal id: %s", terminalID)
		}
		return currentTerminalProbe{
			InsideTmux:  true,
			TmuxSession: "focus-shift",
			TmuxPane:    "focus-shift:0.0",
		}, nil
	}

	inspection, err := service.InspectCurrent()
	if err != nil {
		t.Fatalf("inspect current: %v", err)
	}
	if inspection.Adoptable {
		t.Fatalf("expected inspect current to reject focus drift")
	}
	if inspection.Reason != "focused terminal changed during probe" {
		t.Fatalf("unexpected inspect current reason: %q", inspection.Reason)
	}
	if _, err := service.AdoptCurrent(); err == nil {
		t.Fatalf("expected adopt current to fail when focus changes during probe")
	}
	if len(service.state.Workspaces) != 0 {
		t.Fatalf("expected no workspace to be created after probe focus drift")
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected no new Ghostty window to be created, got %d", fakeGhostty.newWindowCalls)
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

func TestDeletePaneRemovesPaneArtifacts(t *testing.T) {
	service := newTestService(t)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	second, err := service.SplitPane(created.Pane.ID, "up", "")
	if err != nil {
		t.Fatalf("split pane: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
		_ = service.tmux.KillSession(created.Pane.LocalTmuxSession)
		_ = service.tmux.KillSession(second.LocalTmuxSession)
	})

	keepAction := model.NewAction(created.Pane.ID, "agent", "echo keep", "echo keep", model.RiskRead, model.ApprovalNotRequired, model.ActionQueued)
	dropAction := model.NewAction(second.ID, "agent", "echo drop", "echo drop", model.RiskRead, model.ApprovalPending, model.ActionQueued)
	service.actions = []model.Action{keepAction, dropAction}
	service.lastObserved[created.Pane.ID] = time.Now().UTC()
	service.lastObserved[second.ID] = time.Now().UTC()

	if err := service.DeletePane(second.ID); err != nil {
		t.Fatalf("delete pane: %v", err)
	}

	workspace, ok := service.state.Workspaces[created.Workspace.ID]
	if !ok {
		t.Fatalf("expected workspace %s to remain", created.Workspace.ID)
	}
	if len(workspace.PaneIDs) != 1 || workspace.PaneIDs[0] != created.Pane.ID {
		t.Fatalf("unexpected workspace pane ids after delete: %+v", workspace.PaneIDs)
	}
	if _, ok := service.state.Panes[second.ID]; ok {
		t.Fatalf("expected deleted pane %s to be removed from state", second.ID)
	}
	if _, ok := service.lastObserved[second.ID]; ok {
		t.Fatalf("expected deleted pane %s observation state to be removed", second.ID)
	}
	if _, ok := service.lastObserved[created.Pane.ID]; !ok {
		t.Fatalf("expected remaining pane %s observation state to stay", created.Pane.ID)
	}
	if len(service.actions) != 1 || service.actions[0].ID != keepAction.ID {
		t.Fatalf("unexpected actions after pane delete: %+v", service.actions)
	}

	secondAlive, err := service.tmux.HasSession(second.LocalTmuxSession)
	if err != nil {
		t.Fatalf("check deleted pane session: %v", err)
	}
	if secondAlive {
		t.Fatalf("expected deleted pane session %s to be terminated", second.LocalTmuxSession)
	}
	createdAlive, err := service.tmux.HasSession(created.Pane.LocalTmuxSession)
	if err != nil {
		t.Fatalf("check remaining pane session: %v", err)
	}
	if !createdAlive {
		t.Fatalf("expected remaining pane session %s to stay alive", created.Pane.LocalTmuxSession)
	}
}

func TestDeletePaneRemovesWorkspaceWhenLastPaneDeleted(t *testing.T) {
	service := newTestService(t)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.tmux.KillSession(created.Pane.LocalTmuxSession)
	})

	service.actions = []model.Action{
		model.NewAction(created.Pane.ID, "agent", "echo drop", "echo drop", model.RiskRead, model.ApprovalPending, model.ActionQueued),
	}
	service.lastObserved[created.Pane.ID] = time.Now().UTC()

	if err := service.DeletePane(created.Pane.ID); err != nil {
		t.Fatalf("delete last pane: %v", err)
	}

	if _, ok := service.state.Workspaces[created.Workspace.ID]; ok {
		t.Fatalf("expected workspace %s to be removed with its last pane", created.Workspace.ID)
	}
	if _, ok := service.state.Panes[created.Pane.ID]; ok {
		t.Fatalf("expected last pane %s to be removed from state", created.Pane.ID)
	}
	if _, ok := service.lastObserved[created.Pane.ID]; ok {
		t.Fatalf("expected last observed entry for %s to be removed", created.Pane.ID)
	}
	if len(service.actions) != 0 {
		t.Fatalf("expected pane delete to remove related actions, got %+v", service.actions)
	}

	alive, err := service.tmux.HasSession(created.Pane.LocalTmuxSession)
	if err != nil {
		t.Fatalf("check deleted session: %v", err)
	}
	if alive {
		t.Fatalf("expected last pane session %s to be terminated", created.Pane.LocalTmuxSession)
	}
}

func TestDeleteWorkspaceRemovesWorkspaceArtifacts(t *testing.T) {
	service := newTestService(t)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	second, err := service.SplitPane(created.Pane.ID, "up", "")
	if err != nil {
		t.Fatalf("split pane: %v", err)
	}
	other, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create other workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(other.Workspace.ID)
		_ = service.tmux.KillSession(created.Pane.LocalTmuxSession)
		_ = service.tmux.KillSession(second.LocalTmuxSession)
		_ = service.tmux.KillSession(other.Pane.LocalTmuxSession)
	})

	keepAction := model.NewAction(other.Pane.ID, "agent", "echo keep", "echo keep", model.RiskRead, model.ApprovalNotRequired, model.ActionQueued)
	dropActionOne := model.NewAction(created.Pane.ID, "agent", "echo drop-one", "echo drop-one", model.RiskRead, model.ApprovalPending, model.ActionQueued)
	dropActionTwo := model.NewAction(second.ID, "agent", "echo drop-two", "echo drop-two", model.RiskRead, model.ApprovalPending, model.ActionQueued)
	service.actions = []model.Action{dropActionOne, keepAction, dropActionTwo}
	service.lastObserved[created.Pane.ID] = time.Now().UTC()
	service.lastObserved[second.ID] = time.Now().UTC()
	service.lastObserved[other.Pane.ID] = time.Now().UTC()

	if err := service.DeleteWorkspace(created.Workspace.ID); err != nil {
		t.Fatalf("delete workspace: %v", err)
	}

	if _, ok := service.state.Workspaces[created.Workspace.ID]; ok {
		t.Fatalf("expected deleted workspace %s to be removed from state", created.Workspace.ID)
	}
	if _, ok := service.state.Panes[created.Pane.ID]; ok {
		t.Fatalf("expected deleted workspace pane %s to be removed from state", created.Pane.ID)
	}
	if _, ok := service.state.Panes[second.ID]; ok {
		t.Fatalf("expected deleted workspace pane %s to be removed from state", second.ID)
	}
	if _, ok := service.lastObserved[created.Pane.ID]; ok {
		t.Fatalf("expected deleted workspace pane %s observation state to be removed", created.Pane.ID)
	}
	if _, ok := service.lastObserved[second.ID]; ok {
		t.Fatalf("expected deleted workspace pane %s observation state to be removed", second.ID)
	}
	if _, ok := service.state.Workspaces[other.Workspace.ID]; !ok {
		t.Fatalf("expected unrelated workspace %s to remain", other.Workspace.ID)
	}
	if _, ok := service.state.Panes[other.Pane.ID]; !ok {
		t.Fatalf("expected unrelated pane %s to remain", other.Pane.ID)
	}
	if _, ok := service.lastObserved[other.Pane.ID]; !ok {
		t.Fatalf("expected unrelated observation state for %s to remain", other.Pane.ID)
	}
	if len(service.actions) != 1 || service.actions[0].ID != keepAction.ID {
		t.Fatalf("unexpected actions after workspace delete: %+v", service.actions)
	}

	for _, session := range []string{created.Pane.LocalTmuxSession, second.LocalTmuxSession} {
		alive, err := service.tmux.HasSession(session)
		if err != nil {
			t.Fatalf("check deleted workspace session %s: %v", session, err)
		}
		if alive {
			t.Fatalf("expected deleted workspace session %s to be terminated", session)
		}
	}
	otherAlive, err := service.tmux.HasSession(other.Pane.LocalTmuxSession)
	if err != nil {
		t.Fatalf("check unrelated workspace session: %v", err)
	}
	if !otherAlive {
		t.Fatalf("expected unrelated workspace session %s to remain", other.Pane.LocalTmuxSession)
	}
}

func TestClearPaneClearsTmuxHistoryAndCachedSnapshot(t *testing.T) {
	service := newTestService(t)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
	})

	sendMarkerAndWait(t, service, created.Pane.ID, "CLEAR_MARKER_SINGLE")

	cleared, err := service.ClearPane(created.Pane.ID)
	if err != nil {
		t.Fatalf("clear pane: %v", err)
	}
	if strings.Contains(cleared.LastSnapshot, "CLEAR_MARKER_SINGLE") {
		t.Fatalf("expected cleared pane snapshot to drop marker, got %q", cleared.LastSnapshot)
	}

	snapshot, err := waitForSnapshotWithout(t, service, created.Pane.ID, "CLEAR_MARKER_SINGLE")
	if err != nil {
		t.Fatalf("wait for cleared snapshot: %v", err)
	}
	if strings.Contains(snapshot.Text, "CLEAR_MARKER_SINGLE") {
		t.Fatalf("expected pane snapshot to stay cleared, got %q", snapshot.Text)
	}

	paneState := service.state.Panes[created.Pane.ID]
	if strings.Contains(paneState.LastSnapshot, "CLEAR_MARKER_SINGLE") {
		t.Fatalf("expected cached pane snapshot to be refreshed without marker, got %q", paneState.LastSnapshot)
	}
}

func TestClearWorkspaceClearsAllPaneSnapshots(t *testing.T) {
	service := newTestService(t)
	created, err := service.CreateWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	second, err := service.SplitPane(created.Pane.ID, "up", "")
	if err != nil {
		t.Fatalf("split pane: %v", err)
	}
	t.Cleanup(func() {
		_ = service.CloseWorkspace(created.Workspace.ID)
		_ = service.tmux.KillSession(second.LocalTmuxSession)
	})

	sendMarkerAndWait(t, service, created.Pane.ID, "CLEAR_MARKER_WORKSPACE_ONE")
	sendMarkerAndWait(t, service, second.ID, "CLEAR_MARKER_WORKSPACE_TWO")

	cleared, err := service.ClearWorkspace(created.Workspace.ID)
	if err != nil {
		t.Fatalf("clear workspace: %v", err)
	}
	if len(cleared) != 2 {
		t.Fatalf("expected two cleared panes, got %d", len(cleared))
	}

	snapshotOne, err := waitForSnapshotWithout(t, service, created.Pane.ID, "CLEAR_MARKER_WORKSPACE_ONE")
	if err != nil {
		t.Fatalf("wait for cleared first pane snapshot: %v", err)
	}
	if strings.Contains(snapshotOne.Text, "CLEAR_MARKER_WORKSPACE_ONE") {
		t.Fatalf("expected first pane snapshot to stay cleared, got %q", snapshotOne.Text)
	}
	snapshotTwo, err := waitForSnapshotWithout(t, service, second.ID, "CLEAR_MARKER_WORKSPACE_TWO")
	if err != nil {
		t.Fatalf("wait for cleared second pane snapshot: %v", err)
	}
	if strings.Contains(snapshotTwo.Text, "CLEAR_MARKER_WORKSPACE_TWO") {
		t.Fatalf("expected second pane snapshot to stay cleared, got %q", snapshotTwo.Text)
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

func TestReconcileDoesNotRebuildCurrentWindowWorkspace(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)

	workspace := model.NewWorkspace()
	workspace.LaunchMode = model.WorkspaceLaunchModeCurrentWindow
	workspace.GhosttyWindowID = "missing-window"
	workspace.GhosttyTabID = "missing-tab"
	pane := model.NewPane(workspace.ID)
	pane.GhosttyTerminalID = "missing-terminal"
	workspace.PaneIDs = []string{pane.ID}
	service.state.Workspaces[workspace.ID] = workspace
	service.state.Panes[pane.ID] = pane

	workspaces, err := service.Reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("expected one workspace after reconcile, got %d", len(workspaces))
	}
	got := service.state.Workspaces[workspace.ID]
	if got.Status != model.WorkspaceDegraded {
		t.Fatalf("expected current-window workspace to become degraded, got %q", got.Status)
	}
	if got.GhosttyWindowID != "" || got.GhosttyTabID != "" {
		t.Fatalf("expected stale current-window workspace ghostty refs to be cleared, got %+v", got)
	}
	if refreshed := service.state.Panes[pane.ID]; refreshed.GhosttyTerminalID != "" || refreshed.Mode != model.ModeDisconnected {
		t.Fatalf("expected stale pane to disconnect without rebuild, got %+v", refreshed)
	}
	if fakeGhostty.ensureCalls != 0 {
		t.Fatalf("expected reconcile not to call EnsureRunning for current-window workspace, got %d", fakeGhostty.ensureCalls)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected reconcile not to create a new window for current-window workspace, got %d", fakeGhostty.newWindowCalls)
	}
}

func TestStatusSyncClearsMissingCurrentWindowTopology(t *testing.T) {
	service := newTestService(t)
	fakeGhostty := service.ghostty.(*fakeGhosttyClient)

	workspace := model.NewWorkspace()
	workspace.LaunchMode = model.WorkspaceLaunchModeCurrentWindow
	workspace.GhosttyWindowID = "missing-window"
	workspace.GhosttyTabID = "missing-tab"
	pane := model.NewPane(workspace.ID)
	pane.GhosttyTerminalID = "missing-terminal"
	workspace.PaneIDs = []string{pane.ID}
	service.state.Workspaces[workspace.ID] = workspace
	service.state.Panes[pane.ID] = pane

	status, err := service.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.WorkspaceCount != 1 || status.PaneCount != 1 {
		t.Fatalf("unexpected status counts: %+v", status)
	}
	gotWorkspace := service.state.Workspaces[workspace.ID]
	gotPane := service.state.Panes[pane.ID]
	if gotWorkspace.Status != model.WorkspaceDegraded {
		t.Fatalf("expected status sync to degrade stale current-window workspace, got %q", gotWorkspace.Status)
	}
	if gotWorkspace.GhosttyWindowID != "" || gotWorkspace.GhosttyTabID != "" {
		t.Fatalf("expected status sync to clear stale Ghostty refs, got %+v", gotWorkspace)
	}
	if gotPane.GhosttyTerminalID != "" || gotPane.Mode != model.ModeDisconnected {
		t.Fatalf("expected status sync to disconnect stale pane, got %+v", gotPane)
	}
	if fakeGhostty.ensureCalls != 0 {
		t.Fatalf("expected status sync not to call EnsureRunning, got %d", fakeGhostty.ensureCalls)
	}
	if fakeGhostty.newWindowCalls != 0 {
		t.Fatalf("expected status sync not to create a new window, got %d", fakeGhostty.newWindowCalls)
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
	return newTestServiceWithRemote(t, fakeRemoteClient{})
}

func newTestServiceWithRemote(t *testing.T, remoteClient RemoteClient) *Service {
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
		remoteClient,
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

func waitForSnapshotWithout(t *testing.T, service *Service, paneID string, substring string) (model.PaneSnapshot, error) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, err := service.SnapshotPane(paneID)
		if err != nil {
			return model.PaneSnapshot{}, err
		}
		if !strings.Contains(snapshot.Text, substring) {
			return snapshot, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return model.PaneSnapshot{}, context.DeadlineExceeded
}

func sendMarkerAndWait(t *testing.T, service *Service, paneID string, marker string) {
	t.Helper()
	if _, err := service.Claim(paneID, "agent"); err != nil {
		t.Fatalf("claim pane %s: %v", paneID, err)
	}
	markerFile := filepath.Join(t.TempDir(), "marker.txt")
	if err := os.WriteFile(markerFile, []byte(marker+"\n"), 0o600); err != nil {
		t.Fatalf("write marker file for pane %s: %v", paneID, err)
	}
	command := "cat " + execx.ShellQuote(markerFile)
	preview, err := service.PreviewCommand(paneID, "agent", command)
	if err != nil {
		t.Fatalf("preview marker command for pane %s: %v", paneID, err)
	}
	if preview.RequiresApproval {
		if preview.Action == nil {
			t.Fatalf("expected approval action for marker command on pane %s", paneID)
		}
		if _, err := service.Approve(preview.Action.ID); err != nil {
			t.Fatalf("approve marker command for pane %s: %v", paneID, err)
		}
	} else if _, err := service.SendCommand(paneID, "agent", command, ""); err != nil {
		t.Fatalf("send marker to pane %s: %v", paneID, err)
	}
	if _, err := waitForSnapshot(t, service, paneID, marker); err != nil {
		t.Fatalf("wait for marker %s in pane %s: %v", marker, paneID, err)
	}
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
