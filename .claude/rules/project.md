# k8sweep Project Rules

## Model Routing

- **Default: Sonnet 4.6** — for all standard work (edits, tests, reviews, searches)
- **Opus only when needed** — architecture planning, complex debugging, multi-file reasoning
- When spawning subagents, use `model: "sonnet"` unless the task requires deep reasoning

## Code Search

- **Always use Go LSP** for searching Go code (goToDefinition, findReferences, documentSymbol, workspaceSymbol, hover, incomingCalls, outgoingCalls)
- **Never use grep/Grep** for Go symbol searches — LSP is semantically accurate
- Reserve grep only for non-Go files (README, YAML, Makefile) or raw text patterns

## Documentation

- **Update README.md** when adding/changing features, keybindings, CLI flags, or user-facing behavior
- Keep the keybindings table, features list, and usage examples in sync with the code

## Commit Style

- Format: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`
- Always run `go vet ./...` and `go test ./... -race` before committing
