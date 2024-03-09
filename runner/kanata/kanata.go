package kanata

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/rszyma/kanata-tray/os_specific"
	"github.com/rszyma/kanata-tray/runner/tcp_client"
)

// This struct represents a kanata process slot.
// It can be reused multiple times.
// Reusing with different kanata configs/presets is allowed.
type Kanata struct {
	// Prevents race condition when restarting kanata.
	// This must be written to from from outside to free the internal slot.
	ProcessSlotCh chan struct{}

	retCh             chan error // Returns the error returned by `cmd.Wait()`
	ctx               context.Context
	cmd               *exec.Cmd
	logFile           *os.File
	manualTermination bool
	tcpClient         *tcp_client.KanataTcpClient
}

func NewKanataInstance(ctx context.Context) *Kanata {
	return &Kanata{
		ProcessSlotCh: make(chan struct{}, 1),

		retCh:             make(chan error),
		ctx:               ctx,
		cmd:               nil,
		logFile:           nil,
		manualTermination: false,
		tcpClient:         tcp_client.NewTcpClient(),
	}
}

// Terminates the running kanata process, if there is one. It doesn't wait for
// the process to actually stop. If you want to be sure that the process has been
// stopped, try writing to `ProcessSlotCh`. Make sure to pop back the item
// from `ProcessSlotCh` if you want to be able to reuse this struct after that.
func (r *Kanata) StopNonblocking() error {
	if r.cmd != nil {
		if r.cmd.ProcessState != nil {
			// process was already killed from outside?
		} else {
			r.manualTermination = true
			fmt.Println("Killing the currently running kanata process...")
			err := r.cmd.Process.Kill()
			if err != nil {
				return fmt.Errorf("cmd.Process.Kill failed: %v", err)
			}
		}
	}
	return nil
}

func (r *Kanata) RunNonblocking(kanataExecutable string, kanataConfig string, tcpPort int) error {
	err := r.StopNonblocking()
	if err != nil {
		return fmt.Errorf("failed to stop the previous process: %v", err)
	}

	if kanataExecutable == "" {
		var err error
		kanataExecutable, err = exec.LookPath("kanata")
		if err != nil {
			return err
		}
	}

	cfgArg := ""
	if kanataConfig != "" {
		cfgArg = "-c " + kanataConfig
	}

	cmd := exec.CommandContext(r.ctx, kanataExecutable, cfgArg, "--port", fmt.Sprint(tcpPort))
	cmd.SysProcAttr = os_specific.ProcessAttr

	go func() {
		// We're waiting for previous process to be marked as finished in processing loop.
		// We will know that happens when the process slot becomes writable.
		r.ProcessSlotCh <- struct{}{}

		if r.logFile != nil {
			r.logFile.Close()
		}
		r.logFile, err = os.CreateTemp("", "kanata_lastrun_*.log")
		if err != nil {
			r.retCh <- fmt.Errorf("failed to create temp file: %v", err)
			return
		}

		r.cmd = cmd
		r.cmd.Stdout = r.logFile
		r.cmd.Stderr = r.logFile

		fmt.Printf("Running command: %s\n", r.cmd.String())

		err = r.cmd.Start()
		if err != nil {
			r.retCh <- fmt.Errorf("failed to start process: %v", err)
			return
		}

		fmt.Printf("Started kanata (pid=%d)\n", r.cmd.Process.Pid)

		tcpConnectionCtx, cancelTcpConnection := context.WithCancel(r.ctx)
		// Need to wait until kanata boot up and setups the TCP server.
		// 2000 ms is a default boot delay in kanata.
		time.Sleep(time.Millisecond * 2100)

		go func() {
			r.tcpClient.Reconnect <- struct{}{} // this shoudn't block, because reconnect chan should have 1-len buffer
			// Loop in order to reconnect when kanata disconnects us.
			// We might be disconnected if an older version of kanata is used.
			for {
				select {
				case <-tcpConnectionCtx.Done():
					return
				case <-r.tcpClient.Reconnect:
					err := r.tcpClient.Connect(tcpConnectionCtx, tcpPort)
					if err != nil {
						fmt.Printf("Failed to connect to kanata via TCP: %v\n", err)
					}
				}
			}
		}()

		// Send request for layer names. We may or may not get response
		// depending on kanata version). The support for it was implemented in:
		// https://github.com/jtroo/kanata/commit/d66c3c77bcb3acbf58188272177d64bed4130b6e
		err = r.SendClientMessage(tcp_client.ClientMessage{RequestLayerNames: struct{}{}})
		if err != nil {
			fmt.Printf("Failed to send ClientMessage: %v\n", err)
		}

		err = r.cmd.Wait()
		r.cmd = nil
		cancelTcpConnection()
		if r.manualTermination {
			r.manualTermination = false
			r.retCh <- nil
		} else {
			r.retCh <- err
		}
	}()

	return nil
}

func (r *Kanata) LogFile() (string, error) {
	if r.logFile == nil {
		return "", fmt.Errorf("log file doesn't exist")
	}
	return r.logFile.Name(), nil
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
