package guard

import (
	"path/filepath"
	"testing"
)

func TestIsPathWriteProtected(t *testing.T) {
	root := "/home/ryanreadbooks/.tokkibot/refs"
	target := "/home/ryanreadbooks/.tokkibot/refs/test"
	t.Log(filepath.Rel(target, root))
	t.Log(isSubDir(root, target))
}
