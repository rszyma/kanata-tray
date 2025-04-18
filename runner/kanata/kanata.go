package kanata

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sync"
	"time"

	"github.com/labstack/gommon/log"

	"github.com/rszyma/kanata-tray/config"
	"github.com/rszyma/kanata-tray/os_specific"
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

func NewKanataInstance() *Kanata {
	return &Kanata{
		processSlotCh: make(chan struct{}, 1),

		retCh:     make(chan error),
		cmd:       nil,
		tcpClient: tcp_client.NewTcpClient(),
	}
}

func (r *Kanata) RunNonblocking(ctx context.Context, kanataExecutable string, kanataConfig string,
	tcpPort int, hooks config.Hooks, extraArgs []string, logFile *os.File,
) error {
	if kanataExecutable == "" {
		var err error
		kanataExecutable, err = exec.LookPath("kanata")
		if err != nil {
			return err
		}
	}

	cfgArg := ""
	if kanataConfig != "" {
		cfgArg = "-c=" + kanataConfig
	}

	args := append([]string{cfgArg, "--port", fmt.Sprint(tcpPort)}, extraArgs...)
	cmd := cmd(ctx, kanataExecutable, args...)

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
			r.retCh <- fmt.Errorf("hook failed: %s", err)
			return
		}

		log.Infof("Running command: %s", r.cmd.String())

		err = r.cmd.Start()
		if err != nil {
			r.retCh <- fmt.Errorf("failed to start process: %v", err)
			return
		}

		log.Infof("Started kanata (pid=%d)", r.cmd.Process.Pid)

		// Need to wait until kanata boot up and setups the TCP server.
		// 2000 ms is a default start delay in kanata.
		time.Sleep(time.Millisecond * 2500)

		err = runAllBlockingHooks(hooks.PostStart, "post-start")
		if err != nil {
			r.retCh <- fmt.Errorf("hook failed: %s", err)
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

		go func() {
			r.tcpClient.Reconnect <- struct{}{} // this shoudn't block, because reconnect chan should have 1-len buffer
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

		cmdErr := r.cmd.Wait() // block until kanata exits
		r.cmd = nil

		log.Infof("Waiting for all post-start-async hooks to exit")
		<-allPostStartAsyncHooksExitedCh
		log.Infof("All post-start-async hooks exited")

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

// Runs all hooks at the same time, blocking waiting for all of them to finish,
// or they get killed after short timeout.
//
// Returns first encountered error within all hook errors.
//
// `hookName` - stringified hook type e.g. "pre-start".
func runAllBlockingHooks(hooks [][]string, hookName string) error {
	timeout := 5 * time.Second
	// We don't use ctx from outside, because we want to guarantee
	// that the hooks finish normally in case of cancel from outside
	// (e.g. when rapidly switching presets)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	wg := sync.WaitGroup{}
	wg.Add(len(hooks))
	errors := make([]error, len(hooks))
	for i, hook := range hooks {
		log.Infof("Running %s hook [%d] '%s'", hookName, i, hook)
		i := i // fix race condition
		hook := slices.Clone(hook)
		go func() {
			defer wg.Done()
			cmd := cmd(ctx, hook[0], hook[1:]...)
			// TODO: capture stdout/stderr?
			err := cmd.Start()
			if err != nil {
				errors[i] = fmt.Errorf("failed to run %s hook [%d]: %v", hookName, i, err)
				return
			}
			err = cmd.Wait()
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil && ctxErr == context.DeadlineExceeded {
					errors[i] = fmt.Errorf("hook '%s' [%d] was killed because it exceeded maximum allowed runtime for non-async hooks (%s)", hook, i, timeout)
				} else {
					errors[i] = fmt.Errorf("hook '%s' [%d] failed with an error: %v", hook, i, err)
				}
				return
			}
			log.Infof("%s [%d] exited OK", hookName, i)
		}()
	}
	wg.Wait()
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

// `hookName` - stringified hook type e.g. "pre-start".
//
// Returns an error if any error ocurred during startup of any hook.
func runAllAsyncHooks(ctx context.Context, hooks [][]string, hookName string, anyHookErroredCh chan<- error, allHooksExitedCh chan<- struct{}) error {
	anyHookErrored := false
	wg := sync.WaitGroup{}
	wg.Add(len(hooks))
	go func() {
		wg.Wait()
		allHooksExitedCh <- struct{}{}
	}()
	for i, hook := range hooks {
		log.Infof("Running %s hook [%d] '%s'", hookName, i, hook)
		i := i                     // fix race condition
		hook := slices.Clone(hook) // fix race condition
		cmd := cmd(ctx, hook[0], hook[1:]...)
		// TODO: capture stdout/stderr?
		err := cmd.Start()
		if err != nil {
			log.Errorf("Failed to run %s hook [%d]: %v", hookName, i, err)
			return err
		}
		go func() {
			defer wg.Done()
			err := cmd.Wait()
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					log.Warnf("hook '%s' [%d] was killed because of cancel signal: %v", hook, i, ctxErr)
				} else {
					log.Errorf("Hook '%s' [%d] failed with an error: %v", hook, i, err)
				}
				if !anyHookErrored {
					anyHookErrored = true
					anyHookErroredCh <- err
				}
				return
			}
			log.Infof("%s [%d] exited OK", hookName, i)
		}()
	}
	return nil
}

func cmd(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 3 * time.Second
	cmd.SysProcAttr = os_specific.ProcessAttr
	// cmd.Stderr = os.Stderr
	// cmd.Stdout = os.Stdout
	// cmd.Stdin = os.Stdin
	return cmd
}
