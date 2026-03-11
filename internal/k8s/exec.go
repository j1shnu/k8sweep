package k8s

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecOptions contains stream and terminal options for pod exec.
type ExecOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Command       []string
	TTY           bool
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	TerminalSize  remotecommand.TerminalSizeQueue
}

// ExecInPod opens an exec session to a specific pod container.
func ExecInPod(ctx context.Context, client *Client, opts ExecOptions) error {
	if client == nil || client.Clientset() == nil {
		return fmt.Errorf("kubernetes client is not configured")
	}
	if client.RESTConfig() == nil {
		return fmt.Errorf("kubernetes REST config is not available")
	}
	if opts.Namespace == "" || opts.PodName == "" || opts.ContainerName == "" {
		return fmt.Errorf("namespace, pod, and container are required")
	}
	if len(opts.Command) == 0 {
		return fmt.Errorf("exec command is required")
	}

	req := client.Clientset().CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(opts.PodName).
		Namespace(opts.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: opts.ContainerName,
			Command:   opts.Command,
			Stdin:     opts.Stdin != nil,
			Stdout:    opts.Stdout != nil,
			Stderr:    opts.Stderr != nil,
			TTY:       opts.TTY,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(client.RESTConfig(), "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create exec session: %w", err)
	}

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             opts.Stdin,
		Stdout:            opts.Stdout,
		Stderr:            opts.Stderr,
		Tty:               opts.TTY,
		TerminalSizeQueue: opts.TerminalSize,
	})
	if err != nil {
		return fmt.Errorf("exec stream failed: %w", err)
	}
	return nil
}
