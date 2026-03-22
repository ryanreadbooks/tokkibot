package sandbox

import (
	"reflect"
	"testing"
)

func TestWithReadOnlyPaths_AppendsAndKeepsExistingOnEmptyInput(t *testing.T) {
	opt := option{}

	WithReadOnlyPaths("/workspace", "/memory")(&opt)
	WithReadOnlyPaths()(&opt) // should not wipe existing mounts
	WithReadOnlyPaths("", "/refs", "/workspace")(&opt)

	want := []string{"/workspace", "/memory", "/refs"}
	if !reflect.DeepEqual(opt.readOnlyPaths, want) {
		t.Fatalf("unexpected readOnlyPaths: got=%v want=%v", opt.readOnlyPaths, want)
	}
}

func TestWithReadWritePaths_AppendsAndDeduplicates(t *testing.T) {
	opt := option{}

	WithReadWritePaths("/workspace", "/memory")(&opt)
	WithReadWritePaths("/workspace", "/custom")(&opt)

	want := []string{"/workspace", "/memory", "/custom"}
	if !reflect.DeepEqual(opt.readWritePaths, want) {
		t.Fatalf("unexpected readWritePaths: got=%v want=%v", opt.readWritePaths, want)
	}
}
