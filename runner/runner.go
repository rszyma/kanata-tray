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
	tcpClient         KanataTcpClient
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
	r.cmd = nil
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

func (r *KanataRunner) Run(kanataExecutablePath string, kanataConfigPath string) error {
	err := r.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop the previous process: %v", err)
	}

	err = r.CleanupLogs()
	if err != nil {
		// This is non-critical, we can probably continue operating normally.
		fmt.Printf("WARN: process logs cleanup failed: %v\n", err)
	}

	r.logFile, err = os.CreateTemp("", "kanata_lastrun_*.log")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}

	const tcpPort = 5829 // arbitrary number, really

	r.cmd = exec.CommandContext(r.ctx, kanataExecutablePath, "-c", kanataConfigPath, "--port", fmt.Sprint(tcpPort))
	r.cmd.Stdout = r.logFile
	r.cmd.Stderr = r.logFile
	r.cmd.SysProcAttr = os_specific.ProcessAttr

	go func() {
		// We're waiting for previous process to be marked as finished in processing loop.
		// We will know that happens when the process slot becomes writable.
		r.ProcessSlotCh <- struct{}{}

		fmt.Printf("Running command: %s\n", r.cmd.String())

		err = r.cmd.Start()
		if err != nil {
			fmt.Printf("Failed to start process: %v\n", err)
			return
		}

		fmt.Printf("Started kanata (pid=%d)\n", r.cmd.Process.Pid)

		tcpConnectionCtx, cancelTcpConnection := context.WithCancel(r.ctx)
		// Need to wait until kanata boot up and setups the TCP server.
		// 2000 ms is default boot delay in kanata.
		time.Sleep(time.Millisecond * 2100)
		err := r.tcpClient.Connect(tcpConnectionCtx, tcpPort)
		if err != nil {
			fmt.Printf("Failed to connect to kanata via TCP: %v\n", err)
		}

		err = r.cmd.Wait()
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
