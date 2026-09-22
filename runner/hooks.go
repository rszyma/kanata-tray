package runner

import (
	"context"
	"errors"
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
	errs := make([]error, len(hooks))
	for hookIndex, hook := range hooks {
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
				hook[1:],
				map[string]string{},
			)
			// TODO: capture stdout/stderr?
			err := cmd.Start()
			if err != nil {
				errs[hookIndex] = fmt.Errorf("failed to run %s hook [%d]: %v", hookType, n, err)
				return
			}
			err = cmd.Wait()
			if err != nil {
				switch {
				case errors.Is(err, context.DeadlineExceeded):
					errs[hookIndex] = fmt.Errorf(
						"%s hook [%d] was killed because it exceeded maximum allowed runtime for non-async hooks (%s)",
						hookType, n, timeout)
				case errors.Is(err, context.Canceled):
					log.Infof("%s hook [%d] was killed because of cancel signal", hookType, n)
				default:
					errs[hookIndex] = fmt.Errorf("%s hook [%d] failed with an error: %v", hookType, n, err)
				}
				return
			}
			log.Debugf("%s [%d] exited OK", hookType, n)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// `hookType` - stringified hook type e.g. "post-start-async".
func runAllAsyncHooks(ctx context.Context, hooks [][]string, hookType string) (allHooksExitedCh chan struct{}, anyHookErroredCh chan struct{}) {
	allHooksExitedCh = make(chan struct{}, 1)
	anyHookErroredCh = make(chan struct{}, len(hooks))
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
			hook[1:],
			map[string]string{},
		)
		// TODO: capture stdout/stderr?
		err := cmd.Start()
		if err != nil {
			log.Errorf("Failed to run %s hook [%d]: %v", hookType, n, err)
			wg.Done()
			anyHookErroredCh <- struct{}{}
			return allHooksExitedCh, anyHookErroredCh
		}
		go func() {
			defer wg.Done()
			err := cmd.Wait()
			if err != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					log.Infof("%s hook [%d] was killed because of cancel signal", hookType, n)
				} else {
					log.Errorf("%s hook [%d] failed with an error: %v", hookType, n, err)
					anyHookErroredCh <- struct{}{}
				}
				return
			}
			log.Debugf("%s [%d] exited OK", hookType, n)
		}()
	}
	return allHooksExitedCh, anyHookErroredCh
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
