package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
)

const (
	// DefaultTailLines is the default number of log lines to fetch.
	DefaultTailLines int64 = 100
	// maxLogBytes caps the total bytes read from log stream as a safety net.
	maxLogBytes int64 = 1 << 20 // 1 MB
)

// GetPodLogs fetches the last tailLines of logs for a pod container.
func GetPodLogs(ctx context.Context, client *Client, namespace, podName, container string, tailLines int64) ([]string, error) {
	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	}
	req := client.Clientset().CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for %s/%s/%s: %w", namespace, podName, container, err)
	}
	defer stream.Close()

	return ParseLogLines(io.LimitReader(stream, maxLogBytes))
}

// ParseLogLines reads lines from a reader, splitting on newlines.
// Exported for testing.
func ParseLogLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return lines, fmt.Errorf("error reading log stream: %w", err)
	}
	return lines, nil
}
