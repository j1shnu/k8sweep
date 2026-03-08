# k8sweep

A terminal UI for cleaning up Kubernetes pods. Browse, filter, and batch-delete dirty pods (Failed, Completed, Evicted, CrashLoopBackOff, OOMKilled) interactively.

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-blue)

## Features

- Interactive TUI with vim-style navigation (`j`/`k`, `gg`, `G`)
- Real-time pod updates via Kubernetes Watch API (no polling)
- Multi-select pods for batch deletion with confirmation
- Force delete stuck pods (`x`) with graceful shutdown bypass
- Sort columns by name, status, age, restarts, CPU, or memory (`s` to cycle asc/desc)
- Search/filter pods by name in real-time (`/` to search)
- Pod detail panel with labels, annotations, containers, and conditions (`i` to inspect)
- Filter toggle to show only dirty pods
- CPU/memory metrics display (when metrics-server is available)
- Namespace switcher with fuzzy filtering
- Standalone pod warnings (pods with no controller)
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

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` / `↑` / `↓` | Navigate up / down |
| `gg` | Go to first pod |
| `G` | Go to last pod |
| `space` | Toggle pod selection |
| `a` | Select / deselect all |
| `/` | Search pods by name |

### Actions

| Key | Action |
|-----|--------|
| `enter` | Delete selected pods |
| `x` | Force delete selected pods |
| `r` | Refresh pod list |
| `f` | Toggle dirty pod filter |
| `s` | Cycle sort column (asc/desc) |
| `i` | View pod details |
| `n` | Switch namespace |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |

### In search mode

| Key | Action |
|-----|--------|
| Type | Filter pods by name (real-time) |
| `Enter` | Confirm search filter |
| `Esc` | Cancel and clear search |

### In confirmation dialog

| Key | Action |
|-----|--------|
| `y` / `n` | Confirm or cancel |
| `Esc` | Cancel |
| `←` / `→` | Toggle Yes/No |

### In pod detail

| Key | Action |
|-----|--------|
| `j` / `k` | Scroll up / down |
| `i` / `Esc` | Close detail view |

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
  app/          # Bubble Tea state machine (Browsing → Confirming → Searching → Help → Detail)
  k8s/          # Kubernetes client, Watch API, pod operations, metrics, detail fetch
  tui/          # UI components (podlist, header, footer, confirm, namespace, poddetail, help, styles)
  resource/     # Resource interface for extensibility (Jobs, PVCs planned)
```

## License

MIT
