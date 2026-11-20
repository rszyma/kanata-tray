//go:build !windows

package runner

import (
	"bufio"
	"context"
	"testing"
)

// Cancelling the context must send SIGTERM rather than SIGKILL, so that a
// wrapper such as sudo can relay it to kanata.
func TestCmdCancelSendsSIGTERM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	script := `trap 'echo got-term; exit 0' TERM; echo ready; while :; do :; done`
	c := cmd(ctx, nil, nil, "/bin/sh", []string{"-c", script}, nil)
	stdout, err := c.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}

	lines := bufio.NewScanner(stdout)
	if !lines.Scan() || lines.Text() != "ready" {
		t.Fatalf("expected ready line, got %q (err: %v)", lines.Text(), lines.Err())
	}
	cancel()
	if !lines.Scan() || lines.Text() != "got-term" {
		t.Fatalf("process did not receive SIGTERM; got %q (err: %v)", lines.Text(), lines.Err())
	}
	_ = c.Wait()
}
