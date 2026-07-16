package compat

import "sync"

var stableDataRootFuncMu sync.RWMutex

func StableDataRootForTest() func() (string, error) {
	stableDataRootFuncMu.RLock()
	fn := stableDataRootFunc
	stableDataRootFuncMu.RUnlock()
	return fn
}

func SetStableDataRootForTest(fn func() (string, error)) {
	stableDataRootFuncMu.Lock()
	defer stableDataRootFuncMu.Unlock()
	if fn == nil {
		stableDataRootFunc = stableDataRootImpl
		return
	}
	stableDataRootFunc = fn
}
