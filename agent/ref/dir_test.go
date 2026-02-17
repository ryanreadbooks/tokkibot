package ref

import (
	"testing"
)

func TestDir(t *testing.T) {
	t.Log(Fullpath("@refs/o0iowekladsflj"))
}

func TestRandomRefName(t *testing.T) {
	for range 10 {
		t.Log(GetRandomName())
	}
}