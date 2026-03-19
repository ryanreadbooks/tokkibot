//go:build linux

package sandbox

import "testing"

func TestSandboxImpl_Execute(t *testing.T) {
	sandbox := &SandboxImpl{}

	file, err := sandbox.beforeExecute(t.Context())
	t.Log(file)
	t.Log(err)
}
