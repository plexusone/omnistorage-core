package object

import (
	"context"
	"io"
	"testing"
)

func TestAsExtended(t *testing.T) {
	// simpleBackend doesn't implement ExtendedBackend
	var b Backend = &simpleBackend{}

	ext, ok := AsExtended(b)
	if ok {
		t.Error("AsExtended returned true for non-ExtendedBackend")
	}
	if ext != nil {
		t.Error("AsExtended returned non-nil for non-ExtendedBackend")
	}
}

func TestMustExtendedPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustExtended should panic for non-ExtendedBackend")
		}
	}()

	var b Backend = &simpleBackend{}
	_ = MustExtended(b)
}

// simpleBackend implements only Backend, not ExtendedBackend
type simpleBackend struct{}

func (s *simpleBackend) NewWriter(_ context.Context, _ string, _ ...WriterOption) (io.WriteCloser, error) {
	return nil, nil
}

func (s *simpleBackend) NewReader(_ context.Context, _ string, _ ...ReaderOption) (io.ReadCloser, error) {
	return nil, nil
}

func (s *simpleBackend) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (s *simpleBackend) Delete(_ context.Context, _ string) error {
	return nil
}

func (s *simpleBackend) List(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (s *simpleBackend) Close() error {
	return nil
}

// Verify simpleBackend implements Backend
var _ Backend = (*simpleBackend)(nil)
