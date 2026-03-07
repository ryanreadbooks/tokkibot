package tools

import (
	"testing"

	"github.com/ryanreadbooks/tokkibot/component/tool"
)

func TestGlob(t *testing.T) {
	output, err := doGlobInvoke(t.Context(),
		tool.InvokeMeta{},
		&GlobInput{
			Directory: "../",
			Pattern:   "**/*.go",
		})
	t.Log(output)
	t.Log(err)
}
