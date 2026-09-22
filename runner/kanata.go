package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/labstack/gommon/log"

	"github.com/rszyma/kanata-tray/config"
	"github.com/rszyma/kanata-tray/runner/tcp_client"
)

// This struct represents a kanata process slot.
// It can be reused multiple times.
// Reusing with different kanata configs/presets is allowed.
type Kanata struct {
	// Prevents race condition when restarting kanata.
	// This must be written to, to free an internal slot.
	processSlotCh chan struct{}

	// A *single-message* channel, that await an error to be send from the runner.
	// If such error is received, the control is passed down the stack (via this channel),
	// which then calls cancel(), which in turn cancels all goroutines up the stack (in this module).
	//
	// nil is send if the work finished without an error.
	RetCh chan error

	cmd       *exec.Cmd
	tcpClient *tcp_client.KanataTcpClient
}

func NewKanata() *Kanata {
	return &Kanata{
		processSlotCh: make(chan struct{}, 1),

		RetCh:     make(chan error),
		cmd:       nil,
		tcpClient: tcp_client.NewTcpClient(),
	}
}

var KanataCommandFailed = errors.New("kanata exited with an error")
var PostStopHookFailed = errors.New("post-stop hook failed")
var PostStartAsyncHookFailed = errors.New("post-start-async hook failed")

func (r *Kanata) RunNonblocking(ctx context.Context, kanataExecutable string, kanataConfig string,
	tcpPort int, hooks config.Hooks, extraArgs []string, extraEnv map[string]string, logFile *os.File,
) error {
	if kanataExecutable == "" {
		var err error
		// FIXME: kanata.exe on Windows?
		kanataExecutable, err = exec.LookPath("kanata")
		if err != nil {
			return fmt.Errorf("while looking up PATH: %v", err)
		}
	}

	allArgs := []string{}

	if kanataConfig != "" {
		allArgs = append(allArgs, "-c", kanataConfig)
	}

	allArgs = append(allArgs, "--port", fmt.Sprint(tcpPort))

	allArgs = append(allArgs, extraArgs...)

	cmd := cmd(ctx, nil, nil, kanataExecutable, allArgs, extraEnv)

	runWorker := func() error {
		// We're waiting for previous process to be marked as finished.
		// We will know that happens when the process slot becomes writable.
		r.processSlotCh <- struct{}{}
		defer func() {
			<-r.processSlotCh
		}()

		var err error

		r.cmd = cmd
		r.cmd.Stdout = logFile
		r.cmd.Stderr = logFile

		err = runAllBlockingHooks(hooks.PreStart, "pre-start")
		if err != nil {
			return fmt.Errorf("runAllBlockingHooks: %v", err)
		}

		log.Debugf("Running command: %s", r.cmd.String())

		err = r.cmd.Start()
		if err != nil {
			return fmt.Errorf("failed to start process: %v", err)
		}

		log.Infof("Started kanata (pid=%d)", r.cmd.Process.Pid)

		// NOTE: Default wait time in kanata is 2000ms, so we need to wait at least this much.
		// TODO: check if --wait-device-ms delays TCP server start.
		initialConnectErr := r.tcpClient.StartInBackground(ctx, tcpPort, 5000*time.Millisecond)

		select {
		case err := <-initialConnectErr:
			if err == nil {
				// Send request for layer names. We may or may not get response,
				// depending on the kanata version. The support for it was implemented in:
				// https://github.com/jtroo/kanata/commit/d66c3c77bcb3acbf58188272177d64bed4130b6e
				err = r.SendClientMessage(tcp_client.ClientMessage{RequestLayerNames: struct{}{}})
				if err != nil {
					log.Warnf("sending RequestLayerNames failed: %v (kanata old version too old?); icons-to-layers mapping will not be validated.", err)
				}
			} else if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			} else {
				log.Warnf("Couldn't establish connection to kanata via TCP; continuing with reduced functionality; error: %v", err)
			}
		case <-ctx.Done():
			return nil
		}

		err = runAllBlockingHooks(hooks.PostStart, "post-start")
		if err != nil {
			return fmt.Errorf("runAllBlockingHooks: %v", err)
		}

		ctxAsyncHooks, asyncHooksCancel := context.WithCancel(ctx)
		defer asyncHooksCancel()
		allPostStartAsyncHooksExitedCh, anyHookErroredCh := runAllAsyncHooks(ctxAsyncHooks, hooks.PostStartAsync, "post-start-async")

		cmdWaitCh := make(chan error, 1)
		go func() {
			cmdWaitCh <- cmd.Wait()
		}()

		var cmdWaitInterruptedErr error
		select {
		case cmdWaitInterruptedErr = <-cmdWaitCh:
		case <-anyHookErroredCh:
			cmdWaitInterruptedErr = PostStartAsyncHookFailed
		}

		log.Debugf("kanata cmd wait interrupted, cleaning up")

		if len(hooks.PostStartAsync) > 0 {
			asyncHooksCancel()
			log.Debugf("Waiting for all post-start-async hooks to exit")
			<-allPostStartAsyncHooksExitedCh
			log.Debugf("All post-start-async hooks exited")
		}

		err = runAllBlockingHooks(hooks.PostStop, "post-stop")
		if err != nil {
			err1 := fmt.Errorf("%w: %v", PostStopHookFailed, err)
			if cmdWaitInterruptedErr != nil {
				log.Error(err1)
			} else {
				return err1
			}
		}

		if errors.Is(ctx.Err(), context.Canceled) {
			// kill was issued from outside
			return nil
		}

		if cmdWaitInterruptedErr == PostStartAsyncHookFailed {
			return PostStartAsyncHookFailed
		}
		if cmdWaitInterruptedErr != nil {
			// kanata crashed or terminated itself
			return fmt.Errorf("%w: %v", KanataCommandFailed, cmdWaitInterruptedErr)
		}

		return nil
	}

	go func() {
		err := runWorker()
		r.cmd = nil
		r.RetCh <- err
	}()

	return nil
}

func (r *Kanata) ServerMessageCh() <-chan tcp_client.ServerMessage {
	return r.tcpClient.ServerMessageCh()
}

// If currently there's no opened TCP connection, an error will be returned.
func (r *Kanata) SendClientMessage(msg tcp_client.ClientMessage) error {
	timeout := 200 * time.Millisecond
	timer := time.NewTimer(timeout)
	select {
	case <-timer.C:
		return fmt.Errorf("timeouted after %d ms", timeout.Milliseconds())
	case r.tcpClient.ClientMessageCh <- msg:
		if !timer.Stop() {
			<-timer.C
		}
	}
	return nil
}
