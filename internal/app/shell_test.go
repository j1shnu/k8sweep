package app

import (
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
