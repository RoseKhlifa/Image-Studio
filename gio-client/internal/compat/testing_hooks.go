package compat

func StableDataRootForTest() func() (string, error) {
	return stableDataRootFunc
}

func SetStableDataRootForTest(fn func() (string, error)) {
	if fn == nil {
		stableDataRootFunc = stableDataRootImpl
		return
	}
	stableDataRootFunc = fn
}
