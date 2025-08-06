package runner

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/gommon/log"
)

var hookNum atomic.Int32

// Runs all hooks at the same time, blocking waiting for all of them to finish,
// or they get killed after short timeout.
//
// Returns first encountered error within all hook errors.
//
// `hookType` - stringified hook type e.g. "pre-start".
func runAllBlockingHooks(hooks [][]string, hookType string) error {
	timeout := 5 * time.Second
	// We don't use ctx from outside, because we want to guarantee
	// that the hooks finish normally in case of cancel from outside
	// (e.g. when rapidly switching presets)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	wg := sync.WaitGroup{}
	wg.Add(len(hooks))
	errors := make([]error, len(hooks))
	for _, hook := range hooks {
		n := hookNum.Add(1)
		log.Infof("Running %s hook [%d] '%#v'", hookType, n, hook)
		hook := slices.Clone(hook)
		go func() {
			defer wg.Done()
			cmd := cmd(
				ctx,
				makeLogWrapWriter(fmt.Sprintf("hook=%d", n), "&1"),
				makeLogWrapWriter(fmt.Sprintf("hook=%d", n), "&2"),
				hook[0],
				hook[1:]...,
			)
			// TODO: capture stdout/stderr?
			err := cmd.Start()
			if err != nil {
				errors[n] = fmt.Errorf("failed to run %s hook [%d]: %v", hookType, n, err)
				return
			}
			err = cmd.Wait()
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil && ctxErr == context.DeadlineExceeded {
					errors[n] = fmt.Errorf("hook [%d] was killed because it exceeded maximum allowed runtime for non-async hooks (%s)", n, timeout)
				} else {
					errors[n] = fmt.Errorf("hook [%d] failed with an error: %v", n, err)
				}
				return
			}
			log.Infof("%s [%d] exited OK", hookType, n)
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

// `hookType` - stringified hook type e.g. "pre-start".
//
// Returns an error if any error ocurred during startup of any hook.
func runAllAsyncHooks(ctx context.Context, hooks [][]string, hookType string, anyHookErroredCh chan<- error, allHooksExitedCh chan<- struct{}) error {
	anyHookErrored := false
	wg := sync.WaitGroup{}
	wg.Add(len(hooks))
	go func() {
		wg.Wait()
		allHooksExitedCh <- struct{}{}
	}()
	for _, hook := range hooks {
		n := hookNum.Add(1)
		log.Infof("Running %s hook [%d] '%#v'", hookType, n, hook)
		hook := slices.Clone(hook) // fix race condition
		cmd := cmd(
			ctx,
			makeLogWrapWriter(fmt.Sprintf("hook=%d", n), "&1"),
			makeLogWrapWriter(fmt.Sprintf("hook=%d", n), "&2"),
			hook[0],
			hook[1:]...,
		)
		// TODO: capture stdout/stderr?
		err := cmd.Start()
		if err != nil {
			log.Errorf("Failed to run %s hook [%d]: %v", hookType, n, err)
			return err
		}
		go func() {
			defer wg.Done()
			err := cmd.Wait()
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					log.Warnf("hook [%d] was killed because of cancel signal: %v", n, ctxErr)
				} else {
					log.Errorf("Hook [%d] failed with an error: %v", n, err)
				}
				if !anyHookErrored {
					anyHookErrored = true
					anyHookErroredCh <- err
				}
				return
			}
			log.Infof("%s [%d] exited OK", hookType, n)
		}()
	}
	return nil
}

func makeLogWrapWriter(prefixes ...string) io.Writer {
	var prefixesFormatted string
	for _, prefix := range prefixes {
		prefixesFormatted = fmt.Sprintf("%s[%s]", prefixesFormatted, prefix)
	}
	return &writerFunc{func(p []byte) (int, error) {
		s := strings.Trim(string(p), "\n")
		if len(s) > 0 {
			log.Debugf("%s %s", prefixesFormatted, s)
		}
		return len(p), nil
	}}
}

type writerFunc struct {
	writeFunc func(p []byte) (int, error)
}

func (w *writerFunc) Write(p []byte) (int, error) {
	return w.writeFunc(p)
}
