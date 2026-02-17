package process

import (
	"context"
	"sync"
	"time"
)

const (
	exitTimeout = 5 * time.Second
)

type CmdCtxKey string

const (
	RootWgKey CmdCtxKey = "__root_wg_key__"
)

func GetRootWaitGroup(ctx context.Context) *sync.WaitGroup {
	v := ctx.Value(RootWgKey)
	if wg, ok := v.(*sync.WaitGroup); ok {
		return wg
	}

	return nil
}

func GetRootContext() (context.Context, context.CancelFunc, func()) {
	rootWg := &sync.WaitGroup{}
	rootCtx, rootCancel := context.WithCancel(context.Background())
	rootCtx = context.WithValue(rootCtx, RootWgKey, rootWg)

	waitFn := func() {
		exitCtx, exitCancel := context.WithTimeout(context.Background(), exitTimeout)
		defer exitCancel()

		rootWg := GetRootWaitGroup(rootCtx)

		waitDone := make(chan struct{})
		go func() {
			if rootWg != nil {
				rootWg.Wait()
			}
			close(waitDone)
		}()

		select {
		case <-exitCtx.Done():
		case <-waitDone:
		}
	}

	return rootCtx, rootCancel, waitFn
}
