package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/k8s"
	"golang.org/x/term"
	"k8s.io/client-go/tools/remotecommand"
)

var shellCandidates = []string{"/bin/bash", "/bin/sh", "/busybox/sh"}

type podShellExecCommand struct {
	client        *k8s.Client
	contextName   string
	kubeconfig    string
	namespace     string
	podName       string
	containerName string

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	backendUsed string
	shellUsed   string
}

func (m Model) openShellCmd(namespace, podName, containerName string) tea.Cmd {
	podKey := namespace + "/" + podName
	cluster := m.client.GetClusterInfo()
	command := &podShellExecCommand{
		client:        m.client,
		contextName:   cluster.ContextName,
		kubeconfig:    m.client.KubeconfigPath(),
		namespace:     namespace,
		podName:       podName,
		containerName: containerName,
	}
	return tea.Exec(command, func(err error) tea.Msg {
		return PodShellExitedMsg{
			PodKey:    podKey,
			Container: containerName,
			Backend:   command.backendUsed,
			ShellPath: command.shellUsed,
			Err:       err,
		}
	})
}

func (c *podShellExecCommand) SetStdin(r io.Reader)  { c.stdin = r }
func (c *podShellExecCommand) SetStdout(w io.Writer) { c.stdout = w }
func (c *podShellExecCommand) SetStderr(w io.Writer) { c.stderr = w }

func (c *podShellExecCommand) Run() error {
	if c.stdin == nil {
		c.stdin = os.Stdin
	}
	if c.stdout == nil {
		c.stdout = os.Stdout
	}
	if c.stderr == nil {
		c.stderr = os.Stderr
	}

	kubectlErr := c.runWithKubectl()
	if kubectlErr == nil {
		return nil
	}

	clientGoErr := c.runWithClientGo()
	if clientGoErr == nil {
		return nil
	}

	return fmt.Errorf("shell launch failed: kubectl backend: %v; client-go backend: %w", kubectlErr, clientGoErr)
}

func (c *podShellExecCommand) runWithKubectl() error {
	if _, err := exec.LookPath("kubectl"); err != nil {
		return fmt.Errorf("kubectl not found in PATH: %w", err)
	}

	var lastErr error
	for _, shellPath := range shellCandidates {
		args := buildKubectlExecArgs(
			c.contextName,
			c.kubeconfig,
			c.namespace,
			c.podName,
			c.containerName,
			shellPath,
		)
		cmd := exec.Command("kubectl", args...)
		cmd.Stdin = c.stdin
		cmd.Stdout = c.stdout

		var stderrBuf bytes.Buffer
		cmd.Stderr = io.MultiWriter(c.stderr, &stderrBuf)
		err := cmd.Run()
		if err == nil {
			c.backendUsed = "kubectl"
			c.shellUsed = shellPath
			return nil
		}
		if isBenignShellExitError(err) {
			c.backendUsed = "kubectl"
			c.shellUsed = shellPath
			return nil
		}
		lastErr = err
		if isMissingShellError(err, stderrBuf.String()) {
			continue
		}
		return fmt.Errorf("kubectl exec failed: %w", err)
	}

	if lastErr != nil {
		return fmt.Errorf("no supported shell found via kubectl (%w)", lastErr)
	}
	return fmt.Errorf("no supported shell found via kubectl")
}

func buildKubectlExecArgs(contextName, kubeconfigPath, namespace, podName, containerName, shellPath string) []string {
	args := make([]string, 0, 16)
	if kubeconfigPath != "" {
		args = append(args, "--kubeconfig", kubeconfigPath)
	}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}
	args = append(args,
		"exec", "-it",
		"-n", namespace,
		podName,
		"-c", containerName,
		"--", shellPath,
	)
	return args
}

func (c *podShellExecCommand) runWithClientGo() error {
	if c.client == nil || c.client.RESTConfig() == nil {
		return fmt.Errorf("REST config unavailable")
	}

	var (
		lastErr      error
		stdinFile, _ = c.stdin.(*os.File)
		restoreState *term.State
		restoreFD    int
	)

	if stdinFile != nil {
		fd := int(stdinFile.Fd())
		if term.IsTerminal(fd) {
			if state, err := term.MakeRaw(fd); err == nil {
				restoreState = state
				restoreFD = fd
				defer func() {
					_ = term.Restore(restoreFD, restoreState)
				}()
			}
		}
	}

	sizeQueue := newTerminalSizeQueue(c.stdin)
	for _, shellPath := range shellCandidates {
		err := k8s.ExecInPod(context.Background(), c.client, k8s.ExecOptions{
			Namespace:     c.namespace,
			PodName:       c.podName,
			ContainerName: c.containerName,
			Command:       []string{shellPath},
			TTY:           true,
			Stdin:         c.stdin,
			Stdout:        c.stdout,
			Stderr:        c.stderr,
			TerminalSize:  sizeQueue,
		})
		if err == nil {
			c.backendUsed = "client-go"
			c.shellUsed = shellPath
			return nil
		}
		if isBenignShellExitError(err) {
			c.backendUsed = "client-go"
			c.shellUsed = shellPath
			return nil
		}
		lastErr = err
		if isMissingShellError(err, err.Error()) {
			continue
		}
		return fmt.Errorf("client-go exec failed: %w", err)
	}

	if lastErr != nil {
		return fmt.Errorf("no supported shell found via client-go (%w)", lastErr)
	}
	return fmt.Errorf("no supported shell found via client-go")
}

func isMissingShellError(err error, stderr string) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(stderr + " " + err.Error())
	return strings.Contains(text, "executable file not found") ||
		strings.Contains(text, "no such file or directory") ||
		strings.Contains(text, "not found")
}

func isBenignShellExitError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "exit code 130") ||
		strings.Contains(text, "signal: interrupt")
}

type singleTerminalSizeQueue struct {
	size *remotecommand.TerminalSize
	sent bool
}

func newTerminalSizeQueue(stdin io.Reader) remotecommand.TerminalSizeQueue {
	stdinFile, ok := stdin.(*os.File)
	if !ok {
		return nil
	}
	fd := int(stdinFile.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}
	w, h, err := term.GetSize(fd)
	if err != nil {
		return nil
	}
	return &singleTerminalSizeQueue{
		size: &remotecommand.TerminalSize{
			Width:  uint16(w),
			Height: uint16(h),
		},
	}
}

func (q *singleTerminalSizeQueue) Next() *remotecommand.TerminalSize {
	if q == nil || q.sent {
		return nil
	}
	q.sent = true
	return q.size
}
