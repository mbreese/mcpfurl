# Repository Guidelines

## Project Structure & Module Organization
- `cmd/`: Cobra command implementations (`fetch`, `search`, `mcp`, `mcp-http`, config loading).
- `fetchurl/`: Core logic for fetching, converting to Markdown, caching, image download, summarization, and URL policy checks.
- `mcpserver/`: MCP server wiring for stdio/HTTP transport.
- `main.go`: Entrypoint that registers commands.
- `config.toml.default`: Reference config; copy to `config.toml` or point via `MCPFURL_CONFIG`.
- `bin/`: Build outputs created by the Makefile (not checked in).
- `Dockerfile`: Image used for devcontainers/CI-style builds with ChromeDriver available.

## Build, Test, and Development Commands
- `make`: Default build; produces `bin/mcpfurl.linux`.
- `make bin/mcpfurl.macos_arm64` (or other targets): Cross-compile for the listed OS/arch.
- `make run`: Fast local run via `go run main.go`.
- `go test ./...`: Run unit tests when present; add coverage for new logic.
- Devcontainer helpers (if using): `devcontainer up --workspace-folder .`, `devcontainer exec --workspace-folder . bash`.

## Coding Style & Naming Conventions
- Go 1.24; run `gofmt -w` (and `goimports` if available) before submitting.
- Package names are short and lower-case; exported identifiers use PascalCase with doc comments.
- Keep command flag names aligned with config keys (e.g., `--wd-path` ↔ `web_driver_path`).
- Prefer context-aware functions and avoid global state beyond configuration structs.
- New binaries should write under `bin/` to avoid polluting the root.

## Testing Guidelines
- Favor table-driven tests in `_test.go` files; keep fixtures small.
- Use `go test ./...` for unit coverage; isolate integration tests that need ChromeDriver or Google API keys behind build tags or `t.Skip` when unavailable.
- When touching caching, validate behavior against the SQLite-backed cache (`cache.db`).

## Commit & Pull Request Guidelines
- Follow the existing history: concise, imperative subject lines (e.g., “add selectors config”, “update mcp search output”).
- Ensure PRs describe the change, mention new flags/config keys, and note any external dependencies (ChromeDriver, Google Custom Search credentials).
- Include steps to reproduce or run (commands above); attach sample MCP requests/responses or screenshots when UI/HTTP behavior changes.
- Keep unrelated formatting noise out of diffs; prefer separate commits for refactors vs. feature changes.

## Security & Configuration Tips
- Never commit API keys or bearer tokens; load them via env (`MCPFETCH_MASTER_KEY`, `MCPFURL_CONFIG`) or local config files excluded from version control.
- Restrict allowed URLs in config when testing untrusted inputs; use `allow`/`deny` lists to lock down fetch targets.
