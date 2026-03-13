package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jprasad/k8sweep/internal/app"
	"github.com/jprasad/k8sweep/internal/config"
	"github.com/jprasad/k8sweep/internal/k8s"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "")
	flag.BoolVar(showVersion, "v", false, "")

	kubeconfig := flag.String("kubeconfig", "", "")
	flag.StringVar(kubeconfig, "k", "", "")

	context := flag.String("context", "", "")
	flag.StringVar(context, "c", "", "")

	namespace := flag.String("namespace", "", "")
	flag.StringVar(namespace, "n", "", "")

	allNamespaces := flag.Bool("all-namespaces", false, "")
	flag.BoolVar(allNamespaces, "A", false, "")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: k8sweep [flags]\n\nFlags:\n")
		fmt.Fprintf(os.Stderr, "  -v, --version              print version and exit\n")
		fmt.Fprintf(os.Stderr, "  -k, --kubeconfig <path>    path to kubeconfig file (defaults to KUBECONFIG env or ~/.kube/config)\n")
		fmt.Fprintf(os.Stderr, "  -c, --context <name>       kubernetes context to use\n")
		fmt.Fprintf(os.Stderr, "  -n, --namespace <name>     kubernetes namespace (defaults to current context's namespace)\n")
		fmt.Fprintf(os.Stderr, "  -A, --all-namespaces       show pods from all namespaces\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Println("k8sweep " + version)
		return
	}

	k8s.SuppressBenignRuntimeErrors()

	// Load saved preferences
	prefsPath := config.DefaultPath()
	prefs := config.Load(prefsPath)

	// Apply saved namespace/all-namespaces only when no CLI flag overrides it.
	nsOverride := *namespace
	allNS := *allNamespaces
	if nsOverride == "" && !allNS && prefs.Namespace != "" {
		nsExplicit := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "namespace" || f.Name == "n" || f.Name == "all-namespaces" || f.Name == "A" {
				nsExplicit = true
			}
		})
		if !nsExplicit {
			if prefs.Namespace == config.AllNamespacesSentinel {
				allNS = true
			} else {
				nsOverride = prefs.Namespace
			}
		}
	}

	client, err := k8s.NewClient(k8s.ClientConfig{
		KubeconfigPath:    *kubeconfig,
		ContextOverride:   *context,
		NamespaceOverride: nsOverride,
		AllNamespaces:     allNS,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	model := app.NewModel(client, app.WithPreferences(prefs, prefsPath))
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
