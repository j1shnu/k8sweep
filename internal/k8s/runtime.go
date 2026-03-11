package k8s

import (
	"context"
	"errors"
	"io"
	"sync"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var suppressRuntimeErrorOnce sync.Once

// SuppressBenignRuntimeErrors suppresses non-actionable Kubernetes runtime logs.
// Currently this filters io.EOF, which can be emitted when interactive exec
// streams close normally.
func SuppressBenignRuntimeErrors() {
	suppressRuntimeErrorOnce.Do(func() {
		handlers := utilruntime.ErrorHandlers
		utilruntime.ErrorHandlers = []utilruntime.ErrorHandler{
			func(ctx context.Context, err error, msg string, keysAndValues ...interface{}) {
				if err == nil || errors.Is(err, io.EOF) {
					return
				}
				for _, h := range handlers {
					h(ctx, err, msg, keysAndValues...)
				}
			},
		}
	})
}
