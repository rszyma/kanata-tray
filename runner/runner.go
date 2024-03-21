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
	retCh                 chan ItemAndPresetName[error]
	serverMessageCh       chan ItemAndPresetName[tcp_client.ServerMessage]
	clientMessageChannels map[string]chan tcp_client.ClientMessage
	// Maps preset names to runner indices in `runnerPool` and contexts in `instanceWatcherCtxs`.
	activeKanataInstances map[string]int
	// Number of items in channel denotes the number of running kanata instances.
	kanataInstancePool  []*kanata.Kanata
	instanceWatcherCtxs []context.Context
	// Need to have mutex to ensure values in `kanataInstancePool` are not being overwritten
	// while a value from `activeKanataInstances` is still "borrowed".
	instancesMappingLock sync.Mutex
	runnersLimit         int
}

func NewRunner(ctx context.Context) *Runner {
	activeInstancesLimit := 10
	return &Runner{
		retCh:                 make(chan ItemAndPresetName[error]),
		serverMessageCh:       make(chan ItemAndPresetName[tcp_client.ServerMessage]),
		clientMessageChannels: make(map[string]chan tcp_client.ClientMessage),
		activeKanataInstances: make(map[string]int),
		kanataInstancePool:    []*kanata.Kanata{},
		instanceWatcherCtxs:   []context.Context{},
		runnersLimit:          activeInstancesLimit,
	}
}

// Run a new kanata instance from a preset. Blocks until the process is started.
// Calling Run when there's a previous preset running with the the same
// presetName will block until the previous process finishes.
// To stop running preset, caller needs to cancel ctx.
func (r *Runner) Run(ctx context.Context, presetName string, kanataExecutable string, kanataConfig string, tcpPort int) error {
	r.instancesMappingLock.Lock()
	defer r.instancesMappingLock.Unlock()

	var instanceIndex int

	// First check if there's an instance for the given preset already running.
	// If yes, then reuse it. Otherwise reuse free instance if any is available,
	// or create a new Kanata instance.

	if i, ok := r.activeKanataInstances[presetName]; ok {
		// reuse (restart) at index
		instanceIndex = i
	} else if len(r.activeKanataInstances) < len(r.kanataInstancePool) {
		// reuse first free instance
		activeInstanceIndices := []int{}
		for _, i := range r.activeKanataInstances {
			activeInstanceIndices = append(activeInstanceIndices, i)
		}
		sort.Ints(activeInstanceIndices)
		for i := 0; i < len(r.kanataInstancePool); i++ {
			if i >= len(activeInstanceIndices) {
				instanceIndex = i
				break
			}
			if activeInstanceIndices[i] > i {
				// kanataInstancePool at index `i` is unused
				instanceIndex = i
				break
			}
		}
	} else {
		// create new instance
		if len(r.activeKanataInstances) >= r.runnersLimit {
			return fmt.Errorf("active instances limit exceeded")
		}
		r.kanataInstancePool = append(r.kanataInstancePool, kanata.NewKanataInstance())
		instanceIndex = len(r.kanataInstancePool) - 1
	}

	instance := r.kanataInstancePool[instanceIndex]
	err := instance.RunNonblocking(ctx, kanataExecutable, kanataConfig, tcpPort)
	if err != nil {
		return fmt.Errorf("failed to run kanata: %v", err)
	}
	r.activeKanataInstances[presetName] = instanceIndex
	r.clientMessageChannels[presetName] = make(chan tcp_client.ClientMessage)

	go func() {
		<-ctx.Done()
		r.instancesMappingLock.Lock()
		defer r.instancesMappingLock.Unlock()
		delete(r.activeKanataInstances, presetName)
		delete(r.clientMessageChannels, presetName)
	}()

	go func() {
		retCh := instance.RetCh()
		serverMessageCh := instance.ServerMessageCh()
		clientMesasgeCh := r.clientMessageChannels[presetName]
		for {
			select {
			case ret := <-retCh:
				r.retCh <- ItemAndPresetName[error]{
					Item:       ret,
					PresetName: presetName,
				}
				return
			case msg := <-serverMessageCh:
				r.serverMessageCh <- ItemAndPresetName[tcp_client.ServerMessage]{
					Item:       msg,
					PresetName: presetName,
				}
			case msg := <-clientMesasgeCh:
				instance.SendClientMessage(msg)
			}
		}
	}()

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
		return fmt.Errorf("preset with the given name not found")
	}
	return r.kanataInstancePool[presetIndex].SendClientMessage(msg)
}

func (r *Runner) RetCh() <-chan ItemAndPresetName[error] {
	return r.retCh
}

func (r *Runner) ServerMessageCh() <-chan ItemAndPresetName[tcp_client.ServerMessage] {
	return r.serverMessageCh
}

func (r *Runner) LogFile(presetName string) (string, error) {
	r.instancesMappingLock.Lock()
	defer r.instancesMappingLock.Unlock()
	presetIndex, ok := r.activeKanataInstances[presetName]
	if !ok {
		return "", fmt.Errorf("preset with the given name not found")
	}
	return r.kanataInstancePool[presetIndex].LogFile()
}
