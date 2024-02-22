package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/rszyma/kanata-tray/os_specific"
)

type KanataRunner struct {
	RetCh         chan error    // Returns the error returned by `cmd.Wait()`
	ProcessSlotCh chan struct{} // prevent race condition when restarting kanata

	ctx               context.Context
	cmd               *exec.Cmd
	logFile           *os.File
	manualTermination bool
	tcpClient         *KanataTcpClient
}

func NewKanataRunner() KanataRunner {
	return KanataRunner{
		RetCh: make(chan error),
		// 1 denotes max numer of running kanata processes allowed at a time
		ProcessSlotCh: make(chan struct{}, 1),

		ctx:               context.Background(),
		cmd:               nil,
		logFile:           nil,
		manualTermination: false,
		tcpClient:         NewTcpClient(),
	}
}

// Terminates running kanata process, if there is one.
func (r *KanataRunner) Stop() error {
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

func (r *KanataRunner) CleanupLogs() error {
	if r.cmd != nil && r.cmd.ProcessState == nil {
		return fmt.Errorf("tried to cleanup logs while kanata process is still running")
	}

	if r.logFile != nil {
		os.RemoveAll(r.logFile.Name())
		r.logFile.Close()
		r.logFile = nil
	}

	return nil
}

func (r *KanataRunner) RunNonblocking(kanataExecutablePath string, kanataConfigPath string, tcpPort int) error {
	err := r.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop the previous process: %v", err)
	}

	cmd := exec.CommandContext(r.ctx, kanataExecutablePath, "-c", kanataConfigPath, "--port", fmt.Sprint(tcpPort))
	cmd.SysProcAttr = os_specific.ProcessAttr

	go func() {
		// We're waiting for previous process to be marked as finished in processing loop.
		// We will know that happens when the process slot becomes writable.
		r.ProcessSlotCh <- struct{}{}

		err = r.CleanupLogs()
		if err != nil {
			// This is non-critical, we can probably continue operating normally.
			fmt.Printf("WARN: process logs cleanup failed: %v\n", err)
		}

		r.logFile, err = os.CreateTemp("", "kanata_lastrun_*.log")
		if err != nil {
			r.RetCh <- fmt.Errorf("failed to create temp file: %v", err)
			return
		}

		r.cmd = cmd
		r.cmd.Stdout = r.logFile
		r.cmd.Stderr = r.logFile

		fmt.Printf("Running command: %s\n", r.cmd.String())

		err = r.cmd.Start()
		if err != nil {
			r.RetCh <- fmt.Errorf("failed to start process: %v", err)
			return
		}

		fmt.Printf("Started kanata (pid=%d)\n", r.cmd.Process.Pid)

		tcpConnectionCtx, cancelTcpConnection := context.WithCancel(r.ctx)
		// Need to wait until kanata boot up and setups the TCP server.
		// 2000 ms is default boot delay in kanata.
		time.Sleep(time.Millisecond * 2100)

		go func() {
			r.tcpClient.reconnect <- struct{}{} // this shoudn't block, because reconnect chan should have 1-len buffer
			// Loop in order to reconnect when kanata disconnects us.
			// We might be disconnected if an older version of kanata is used.
			for {
				select {
				case <-tcpConnectionCtx.Done():
					return
				case <-r.tcpClient.reconnect:
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
		err = r.SendClientMessage(ClientMessage{RequestLayerNames: struct{}{}})
		if err != nil {
			fmt.Printf("Failed to send ClientMessage: %v\n", err)
		}

		err = r.cmd.Wait()
		r.cmd = nil
		cancelTcpConnection()
		if r.manualTermination {
			r.manualTermination = false
			r.RetCh <- nil
		} else {
			r.RetCh <- err
		}
	}()

	return nil
}

func (r *KanataRunner) LogFile() (string, error) {
	if r.logFile == nil {
		return "", fmt.Errorf("log file doesn't exist")
	}
	return r.logFile.Name(), nil
}

func (r *KanataRunner) ServerMessageCh() chan ServerMessage {
	return r.tcpClient.ServerMessageCh
}

// If currently there's no opened TCP connection, an error will be returned.
func (r *KanataRunner) SendClientMessage(msg ClientMessage) error {
	timeout := 200 * time.Millisecond
	timer := time.NewTimer(timeout)
	select {
	case <-timer.C:
		return fmt.Errorf("timeouted after %d ms", timeout.Milliseconds())
	case r.tcpClient.clientMessageCh <- msg:
		if !timer.Stop() {
			<-timer.C
		}
	}
	return nil
}
