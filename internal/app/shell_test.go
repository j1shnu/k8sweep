package app

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildKubectlExecArgs_WithContextAndKubeconfig(t *testing.T) {
	args := buildKubectlExecArgs(
		"stg-cluster",
		"/tmp/kubeconfig.yaml",
		"payments",
		"api-123",
		"main",
		"/bin/sh",
	)

	assert.Equal(t, []string{
		"--kubeconfig", "/tmp/kubeconfig.yaml",
		"--context", "stg-cluster",
		"exec", "-it",
		"-n", "payments",
		"api-123",
		"-c", "main",
		"--", "/bin/sh",
	}, args)
}

func TestBuildKubectlExecArgs_WithoutOptionalFlags(t *testing.T) {
	args := buildKubectlExecArgs(
		"",
		"",
		"default",
		"pod-a",
		"sidecar",
		"/bin/bash",
	)

	assert.Equal(t, []string{
		"exec", "-it",
		"-n", "default",
		"pod-a",
		"-c", "sidecar",
		"--", "/bin/bash",
	}, args)
}

func TestIsBenignShellExitError(t *testing.T) {
	assert.True(t, isBenignShellExitError(errors.New("exec stream failed: command terminated with exit code 130")))
	assert.True(t, isBenignShellExitError(errors.New("signal: interrupt")))
	assert.False(t, isBenignShellExitError(errors.New("exec stream failed: command terminated with exit code 1")))
	assert.False(t, isBenignShellExitError(nil))
}
