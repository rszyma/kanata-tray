package runner

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/rszyma/kanata-tray/os_specific"
)

type KanataRunner struct {
	RetCh             chan error    // Returns the error returned by `cmd.Wait()`
	ProcessSlotCh     chan struct{} // prevent race condition when restarting kanata
	cmd               *exec.Cmd
	logFile           *os.File
	manualTermination bool
}

func NewKanataRunner() KanataRunner {
	return KanataRunner{
		RetCh: make(chan error),
		// 1 denotes max numer of running kanata processes allowed at a time
		ProcessSlotCh: make(chan struct{}, 1),

		cmd:               nil,
		logFile:           nil,
		manualTermination: false,
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
	r.cmd = exec.Command(kanataExecutablePath, "-c", kanataConfigPath)
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

		err := r.cmd.Wait()
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
