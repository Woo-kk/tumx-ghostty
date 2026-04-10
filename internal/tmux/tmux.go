package tmux

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Woo-kk/tmux-ghostty/internal/execx"
)

const (
	defaultTimeout = 10 * time.Second
)

type Client struct {
	runner *execx.Runner
}

func New(runner *execx.Runner) *Client {
	return &Client{runner: runner}
}

func (c *Client) HasSession(name string) (bool, error) {
	result, err := c.run(defaultTimeout, "has-session", "-t", name)
	if err != nil {
		if tmuxErrorContains(result, err, "can't find session") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) NewSession(name string) error {
	_, err := c.run(defaultTimeout, "new-session", "-d", "-A", "-s", name)
	return err
}

func (c *Client) KillSession(name string) error {
	result, err := c.run(defaultTimeout, "kill-session", "-t", name)
	if err != nil {
		if tmuxErrorContains(result, err, "can't find session") {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) ClearPane(target string) error {
	target = normalizeTarget(target)
	_, err := c.run(defaultTimeout, "clear-history", "-t", target)
	return err
}

func (c *Client) SendKeys(target string, text string) error {
	target = normalizeTarget(target)
	if text != "" {
		if err := c.SendText(target, text); err != nil {
			return err
		}
	}
	_, err := c.run(defaultTimeout, "send-keys", "-t", target, "Enter")
	return err
}

func (c *Client) SendText(target string, text string) error {
	target = normalizeTarget(target)
	if text == "" {
		return nil
	}
	_, err := c.run(defaultTimeout, "send-keys", "-t", target, "-l", text)
	return err
}

func (c *Client) SendCtrlC(target string) error {
	_, err := c.run(defaultTimeout, "send-keys", "-t", normalizeTarget(target), "C-c")
	return err
}

func (c *Client) CapturePane(target string, lines int) (string, error) {
	if lines <= 0 {
		lines = 500
	}
	result, err := c.run(defaultTimeout, "capture-pane", "-p", "-J", "-t", normalizeTarget(target), "-S", fmt.Sprintf("-%d", lines))
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

func (c *Client) CurrentCommand(target string) (string, error) {
	result, err := c.run(defaultTimeout, "display-message", "-p", "-t", normalizeTarget(target), "#{pane_current_command}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (c *Client) TargetAlive(target string) (bool, error) {
	result, err := c.run(defaultTimeout, "display-message", "-p", "-t", normalizeTarget(target), "#{pane_id}")
	if err != nil {
		if tmuxErrorContains(result, err, "can't find pane", "can't find window", "can't find session") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) AttachCommand(session string) string {
	return "exec tmux attach-session -t " + execx.ShellQuote(session)
}

func (c *Client) run(timeout time.Duration, args ...string) (execx.Result, error) {
	return c.runner.Run(context.Background(), timeout, "tmux", args...)
}

func normalizeTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if strings.Contains(target, ":") || strings.HasPrefix(target, "%") {
		return target
	}
	return target + ":0.0"
}

func tmuxErrorContains(result execx.Result, err error, fragments ...string) bool {
	text := result.Stderr
	if err != nil {
		if text != "" {
			text += "\n"
		}
		text += err.Error()
	}
	for _, fragment := range fragments {
		if strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}
