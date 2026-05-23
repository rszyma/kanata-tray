package runner

import (
	"context"
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

	retCh     chan error // Returns the error returned by `cmd.Wait()`
	cmd       *exec.Cmd
	tcpClient *tcp_client.KanataTcpClient
}

func NewKanata() *Kanata {
	return &Kanata{
		processSlotCh: make(chan struct{}, 1),

		retCh:     make(chan error),
		cmd:       nil,
		tcpClient: tcp_client.NewTcpClient(),
	}
}

func (r *Kanata) RunNonblocking(ctx context.Context, kanataExecutable string, kanataConfig string,
	tcpPort int, hooks config.Hooks, extraArgs []string, extraEnv map[string]string, logFile *os.File,
) error {
	if kanataExecutable == "" {
		var err error
		// FIXME: kanata.exe on Windows?
		kanataExecutable, err = exec.LookPath("kanata")
		if err != nil {
			return err
		}
	}

	allArgs := []string{}

	if kanataConfig != "" {
		allArgs = append(allArgs, "-c", kanataConfig)
	}

	allArgs = append(allArgs, "--port", fmt.Sprint(tcpPort))

	allArgs = append(allArgs, extraArgs...)

	cmd := cmd(ctx, nil, nil, kanataExecutable, allArgs, extraEnv)

	go func() {
		selfCtx, selfCancel := context.WithCancelCause(ctx)
		defer selfCancel(nil)

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
			r.retCh <- fmt.Errorf("runAllBlockingHooks: %s", err)
			return
		}

		log.Infof("Running command: %s", r.cmd.String())

		err = r.cmd.Start()
		if err != nil {
			r.retCh <- fmt.Errorf("failed to start process: %v", err)
			return
		}

		log.Infof("Started kanata (pid=%d)", r.cmd.Process.Pid)

		// Short poll for kanata tcp server to be up.
		// Default wait time in kanata is 2000ms, but can be disabled with --nodelay, so hardcoding delay here is not good.
		online := false
		deadline := time.Now().Add(time.Millisecond * 5000)
		for time.Now().Before(deadline) {
			ctx, cancel := context.WithTimeout(selfCtx, 25*time.Millisecond)
			err := r.tcpClient.Connect(ctx, tcpPort)
			if err == nil {
				online = true
				defer cancel() // yes, this is a defer in a for loop. It should clean up only after the whole function returns.
				break
			}
			cancel()
			time.Sleep(25 * time.Millisecond)
		}
		if !online {
			log.Errorf("Waiting for kanata TCP server timed out. Bug?")
		}

		go func() {
			// Loop in order to reconnect when kanata disconnects us.
			// We might be disconnected if an older version of kanata is used.
			for {
				select {
				case <-selfCtx.Done():
					return
				case <-r.tcpClient.Reconnect:
					err := r.tcpClient.Connect(selfCtx, tcpPort)
					if err != nil {
						log.Errorf("Failed to connect to kanata via TCP: %v", err)
					}
				}
			}
		}()

		// Send request for layer names. We may or may not get response
		// depending on kanata version). The support for it was implemented in:
		// https://github.com/jtroo/kanata/commit/d66c3c77bcb3acbf58188272177d64bed4130b6e
		err = r.SendClientMessage(tcp_client.ClientMessage{RequestLayerNames: struct{}{}})
		if err != nil {
			log.Errorf("Failed to send ClientMessage: %v", err)
			// this is non-critical, so we continue
		}

		err = runAllBlockingHooks(hooks.PostStart, "post-start")
		if err != nil {
			r.retCh <- fmt.Errorf("runAllBlockingHooks: %s", err)
			return
		}
		anyPostStartAsyncHookErroredCh := make(chan error, 1)
		allPostStartAsyncHooksExitedCh := make(chan struct{}, 1)
		err = runAllAsyncHooks(selfCtx, hooks.PostStartAsync, "post-start-async", anyPostStartAsyncHookErroredCh, allPostStartAsyncHooksExitedCh)
		if err != nil {
			r.retCh <- fmt.Errorf("hook failed: %s", err)
			return
		}

		go func() {
			select {
			case <-selfCtx.Done():
				return
			case err := <-anyPostStartAsyncHookErroredCh:
				log.Errorf("An async hook errored, stopping preset.")
				selfCancel(err)
			}
		}()

		cmdErr := r.cmd.Wait() // block until kanata exits
		r.cmd = nil

		if len(hooks.PostStartAsync) > 0 {
			log.Infof("Waiting for all post-start-async hooks to exit")
		}
		<-allPostStartAsyncHooksExitedCh
		if len(hooks.PostStartAsync) > 0 {
			log.Infof("All post-start-async hooks exited")
		}

		err = runAllBlockingHooks(hooks.PostStop, "post-stop")
		if err != nil {
			r.retCh <- fmt.Errorf("hook failed: %s", err)
			return
		}

		selfCtxErr := selfCtx.Err()
		if selfCtxErr != nil && selfCtxErr != context.DeadlineExceeded && selfCtxErr != context.Canceled {
			// must be an error from async hook
			r.retCh <- selfCtxErr
			return
		}

		if ctx.Err() == context.Canceled {
			// kill was issued from outside
			r.retCh <- nil
			return
		}

		if cmdErr != nil {
			// kanata crashed or terminated itself
			r.retCh <- cmdErr
			return
		}

		r.retCh <- nil
	}()

	return nil
}

func (r *Kanata) RetCh() <-chan error {
	return r.retCh
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
