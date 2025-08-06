package runner

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/labstack/gommon/log"
)

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
	for i, hook := range hooks {
		log.Infof("Running %s hook [%d] '%#v'", hookType, i, hook)
		i := i // fix race condition
		hook := slices.Clone(hook)
		go func() {
			defer wg.Done()
			cmd := cmd(ctx, nil, nil, hook[0], hook[1:]...)
			// TODO: capture stdout/stderr?
			err := cmd.Start()
			if err != nil {
				errors[i] = fmt.Errorf("failed to run %s hook [%d]: %v", hookType, i, err)
				return
			}
			err = cmd.Wait()
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil && ctxErr == context.DeadlineExceeded {
					errors[i] = fmt.Errorf("hook [%d] was killed because it exceeded maximum allowed runtime for non-async hooks (%s)", i, timeout)
				} else {
					errors[i] = fmt.Errorf("hook [%d] failed with an error: %v", i, err)
				}
				return
			}
			log.Infof("%s [%d] exited OK", hookType, i)
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
	for i, hook := range hooks {
		log.Infof("Running %s hook [%d] '%#v'", hookType, i, hook)
		i := i                     // fix race condition
		hook := slices.Clone(hook) // fix race condition
		cmd := cmd(ctx, nil, nil, hook[0], hook[1:]...)
		// TODO: capture stdout/stderr?
		err := cmd.Start()
		if err != nil {
			log.Errorf("Failed to run %s hook [%d]: %v", hookType, i, err)
			return err
		}
		go func() {
			defer wg.Done()
			err := cmd.Wait()
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					log.Warnf("hook [%d] was killed because of cancel signal: %v", i, ctxErr)
				} else {
					log.Errorf("Hook [%d] failed with an error: %v", i, err)
				}
				if !anyHookErrored {
					anyHookErrored = true
					anyHookErroredCh <- err
				}
				return
			}
			log.Infof("%s [%d] exited OK", hookType, i)
		}()
	}
	return nil
}
