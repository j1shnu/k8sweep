package k8s

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func TestSuppressBenignRuntimeErrors_FiltersEOF(t *testing.T) {
	origHandlers := utilruntime.ErrorHandlers
	t.Cleanup(func() {
		utilruntime.ErrorHandlers = origHandlers
		suppressRuntimeErrorOnce = sync.Once{}
	})

	calls := 0
	utilruntime.ErrorHandlers = []utilruntime.ErrorHandler{
		func(context.Context, error, string, ...interface{}) {
			calls++
		},
	}

	suppressRuntimeErrorOnce = sync.Once{}
	SuppressBenignRuntimeErrors()

	for _, h := range utilruntime.ErrorHandlers {
		h(context.Background(), io.EOF, "eof")
	}
	assert.Equal(t, 0, calls)
}

func TestSuppressBenignRuntimeErrors_PassesRealErrors(t *testing.T) {
	origHandlers := utilruntime.ErrorHandlers
	t.Cleanup(func() {
		utilruntime.ErrorHandlers = origHandlers
		suppressRuntimeErrorOnce = sync.Once{}
	})

	calls := 0
	utilruntime.ErrorHandlers = []utilruntime.ErrorHandler{
		func(context.Context, error, string, ...interface{}) {
			calls++
		},
	}

	suppressRuntimeErrorOnce = sync.Once{}
	SuppressBenignRuntimeErrors()

	for _, h := range utilruntime.ErrorHandlers {
		h(context.Background(), errors.New("boom"), "boom")
	}
	assert.Equal(t, 1, calls)
}
