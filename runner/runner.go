package runner

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/rszyma/kanata-tray/runner/kanata"
	"github.com/rszyma/kanata-tray/runner/tcp_client"
)

// An item and the preset name for the associated runner.
type ItemAndPresetName[T any] struct {
	Item       T
	PresetName string
}

type Runner struct {
	retCh           chan ItemAndPresetName[error]
	ProcessSlotCh   chan ItemAndPresetName[struct{}]
	serverMessageCh chan ItemAndPresetName[tcp_client.ServerMessage]
	clientMessageCh chan ItemAndPresetName[tcp_client.ClientMessage]
	// Maps preset names to runner indices in `runnerPool` and contexts in `instanceWatcherCtxs`.
	activeKanataInstances map[string]int
	kanataInstancePool    []*kanata.Kanata
	instanceWatcherCtxs   []context.Context
	// Need to have mutex to ensure values in `kanataInstancePool` are not being overwritten
	// while a value from `activeKanataInstances` is still "borrowed".
	instancesMappingLock sync.Mutex
	concurrent           bool
	runnersLimit         int
	ctx                  context.Context
}

func NewRunner(ctx context.Context, concurrent bool) *Runner {
	activeInstancesLimit := 10
	return &Runner{
		retCh:                 make(chan ItemAndPresetName[error], activeInstancesLimit),
		ProcessSlotCh:         make(chan ItemAndPresetName[struct{}], activeInstancesLimit),
		serverMessageCh:       make(chan ItemAndPresetName[tcp_client.ServerMessage], activeInstancesLimit),
		clientMessageCh:       make(chan ItemAndPresetName[tcp_client.ClientMessage], activeInstancesLimit),
		activeKanataInstances: make(map[string]int),
		kanataInstancePool:    []*kanata.Kanata{},
		instanceWatcherCtxs:   []context.Context{},
		concurrent:            concurrent,
		runnersLimit:          activeInstancesLimit,
		ctx:                   ctx,
	}
}

// Run a new kanata instance from a preset.
// Blocks until the process is started.
//
// Depending on the value of `concurrent`, it will either add a new runner to pool
// (or reuse unused runner) or first stop the running instances, and then run
// the a one it's place. Will fail if active instances limit were to be exceeded.
func (r *Runner) Run(presetName string, kanataExecutable string, kanataConfig string, tcpPort int) error {
	r.instancesMappingLock.Lock()
	defer r.instancesMappingLock.Unlock()

	if r.concurrent {
		// Stop all
		for k, i := range r.activeKanataInstances {
			_ = r.kanataInstancePool[i].StopNonblocking()
			delete(r.activeKanataInstances, k)
		}
	}

	var instanceIndex int

	// First check if there's an instance for the given preset already running.
	// If yes, then reuse it. Otherwise reuse free instance if any is available,
	// or create a new Kanata instance.

	if i, ok := r.activeKanataInstances[presetName]; ok {
		// reuse (restart) at index
		_ = r.kanataInstancePool[i].StopNonblocking()
		instanceIndex = i
	} else if len(r.activeKanataInstances) < len(r.kanataInstancePool) {
		// reuse first free instance
		activeInstanceIndices := []int{}
		for _, i := range r.activeKanataInstances {
			activeInstanceIndices = append(activeInstanceIndices, i)
		}
		sort.Ints(activeInstanceIndices)
		for i := 0; i < len(r.kanataInstancePool); i++ {
			if activeInstanceIndices[i] != i {
				// kanataInstancePool at index `i` is unused
				instanceIndex = i
				break
			}
		}
	} else {
		// create new instance
		if r.runnersLimit < len(r.activeKanataInstances) {
			return fmt.Errorf("active instances limit exceeded")
		}
		r.kanataInstancePool = append(r.kanataInstancePool, kanata.NewKanataInstance(r.ctx))
		instanceIndex = len(r.kanataInstancePool) - 1
	}

	instance := r.kanataInstancePool[instanceIndex]
	err := instance.RunNonblocking(kanataExecutable, kanataConfig, tcpPort)
	if err != nil {
		return fmt.Errorf("failed to run kanata: %v", err)
	}
	r.activeKanataInstances[presetName] = instanceIndex
	return nil
}

func (r *Runner) StopNonblocking(presetName string) error {
	r.instancesMappingLock.Lock()
	defer r.instancesMappingLock.Unlock()
	i, ok := r.activeKanataInstances[presetName]
	if !ok {
		return fmt.Errorf("preset with the provided name is not running")
	}
	err := r.kanataInstancePool[i].StopNonblocking()
	if err != nil {
		return err
	}
	delete(r.activeKanataInstances, presetName) // should this be before or after checking for error?
	return nil
}

// An error will be returned if a preset doesn't exists or there's currently no
// opened TCP connection for the given preset.
//
// FIXME: message can be sent to a wrong kanata process during live-reloading
// if a preset has been changed but there's a preset with the same name as in
// previous kanata-tray configuration. Unlikely to ever happen though
// (also live-reloading is not implemented at the time of writing).
func (r *Runner) SendClientMessage(presetName string, msg tcp_client.ClientMessage) error {
	r.instancesMappingLock.Lock()
	defer r.instancesMappingLock.Unlock()
	presetIndex, ok := r.activeKanataInstances[presetName]
	if !ok {
		return fmt.Errorf("preset with the given nam not found")
	}
	return r.kanataInstancePool[presetIndex].SendClientMessage(msg)
}

func (r *Runner) RetCh() <-chan ItemAndPresetName[error] {
	return r.retCh
}

func (r *Runner) ServerMessageCh() <-chan ItemAndPresetName[tcp_client.ServerMessage] {
	return r.serverMessageCh
}
