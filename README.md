# k8sweep

A terminal UI for cleaning up Kubernetes pods. Browse, filter, and batch-delete dirty pods (Failed, Completed, Evicted, CrashLoopBackOff, OOMKilled) interactively.

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-blue)

## Features

- Interactive TUI with vim-style navigation
- Multi-select pods for batch deletion with confirmation
- Filter toggle to show only dirty pods
- Namespace switcher with fuzzy filtering
- Auto-refresh every 10 seconds
- Status-colored pod list (red=Failed, cyan=Running, gray=Completed, orange=Evicted)
- Respects `KUBECONFIG` env, `~/.kube/config`, and current context

## Installation

### From GitHub Releases

Download the latest binary for your platform from the [Releases](https://github.com/j1shnu/k8sweep/releases) page.

```bash
# Example: Linux amd64
curl -LO https://github.com/j1shnu/k8sweep/releases/latest/download/k8sweep_linux_amd64.tar.gz
tar xzf k8sweep_linux_amd64.tar.gz
sudo mv k8sweep /usr/local/bin/
```

Verify with `k8sweep --version`.

### From source

```bash
git clone https://github.com/j1shnu/k8sweep.git
cd k8sweep
make build
```

Binary will be at `bin/k8sweep`.

### Go install

```bash
go install github.com/jprasad/k8sweep@latest
```

## Usage

```bash
# Use current kubeconfig context
k8sweep

# Specify kubeconfig, context, or namespace
k8sweep --kubeconfig /path/to/config
k8sweep --context my-cluster
k8sweep --namespace kube-system
```

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate down / up |
| `space` | Toggle pod selection |
| `a` | Select / deselect all |
| `enter` | Delete selected pods |
| `f` | Toggle dirty pod filter |
| `n` | Switch namespace |
| `r` | Refresh pod list |
| `q` / `Ctrl+C` | Quit |

### In confirmation dialog

| Key | Action |
|-----|--------|
| `y` | Confirm deletion |
| `n` / `Esc` | Cancel |
| `←` / `→` | Toggle Yes/No |

### In namespace switcher

| Key | Action |
|-----|--------|
| Type | Filter namespaces |
| `↑` / `↓` | Navigate list |
| `Enter` | Select namespace |
| `Esc` | Cancel |

## What are "dirty" pods?

k8sweep considers these pod statuses as dirty — safe to clean up:

| Status | Detection |
|--------|-----------|
| **Completed** | Pod phase is `Succeeded` |
| **Failed** | Pod phase is `Failed` |
| **Evicted** | Pod status reason is `Evicted` |
| **CrashLoopBackOff** | Container waiting with reason `CrashLoopBackOff` |
| **OOMKilled** | Container terminated with reason `OOMKilled` (only if not currently running) |

## Development

```bash
make build      # Build binary
make run        # Run directly
make test       # Run tests with race detector
make coverage   # Show test coverage
make lint       # Run golangci-lint
make clean      # Clean build artifacts
```

## Architecture

```
internal/
  app/          # Bubble Tea state machine (Browsing → Confirming → SwitchingNamespace)
  k8s/          # Kubernetes client, pod operations, status derivation
  tui/          # UI components (podlist, header, footer, confirm, namespace, styles)
  resource/     # Resource interface for extensibility (Jobs, PVCs planned)
```

## License

MIT
