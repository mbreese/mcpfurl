mcpfurl
===

mcpfurl is a Model Context Protocol (MCP) server that can fetch web pages, download images/binaries, and perform Google Custom Search queries. It is packaged as a single CLI that can run either over stdio or streaming-http.

## Highlights

- Fetch full webpages, convert them to Markdown, and stream them back to the MCP client.
- Download images or other binary assets and return them as base64 payloads.
- Perform Google Custom Search queries and respond with either JSON or Markdown summaries.
- Optional SQLite-backed search cache with configurable TTLs.
- HTTP mode can be locked down with a bearer token (`MCPFETCH_MASTER_KEY`).

## Getting Started

* **Easy mode** -  Use the Docker container, which is located at: https://ghcr.io/mbreese/mcpfurl

* **Or...** - Download binary from https://github.com/mbreese/mcpfurl Releases (or an Action workflow run: https://github.com/mbreese/mcpfurl/actions/workflows/go.yml)

* **More difficult** - You can build the project yourself with:

```bash
make
```

But, it is easier to run in a Docker container, especially if you need to also install the Chromedriver headless Chrome browswer.

### Running over stdio

```bash
./mcpfurl mcp --wd-path /usr/bin/chromedriver --google-cx <cx> --google-key <api-key>
```

### Running over HTTP

```bash
./mcpfurl mcp-http --addr 127.0.0.1 --port 8080 --master-key supersecret
# or pick up the same key via MCPFETCH_MASTER_KEY or config.toml
```

When `--master-key` (or `MCPFETCH_MASTER_KEY`) is set, every request to `/mcp` or `/` must include `Authorization: Bearer <value>` or the server returns `401 Unauthorized`.

## Configuration

Configuration values can come from three places, in the following precedence order:

1. CLI flags.
2. Environment variables (e.g., `MCPFURL_CONFIG`, `MCPFETCH_MASTER_KEY`).
3. `config.toml` (or another file pointed to by `MCPFURL_CONFIG`).

See `config.toml.default` for all available options:

```toml
[mcpfurl]
web_driver_port = 9515
web_driver_path = "/usr/bin/chromedriver"
use_pandoc = false
search_engine = "google_custom"
allow = []
disallow = []

[http]
addr = "0.0.0.0"
port = 8080
master_key = ""

[cache]
db_path = "cache.db"
expires = "14d"

[google_custom]
cx = ""
key = ""
```

Only the settings you override need to be present in your config file. The CLI flags mirror these names (`--wd-port`, `--search-cache`, etc.). Set `allow`/`disallow` under `[mcpfurl]` to control which URLs the server may fetch; when `allow` is empty every URL is permitted unless a `disallow` glob matches.

## Dependencies

- ChromeDriver (or another Selenium-compatible WebDriver) must be installed and reachable via `--wd-path`.
- Google Custom Search API credentials (`google_custom.cx` and `google_custom.key`) are required for search functionality.
- SQLite is used for caching search results (optional but recommended).

## Dev notes

I'm using devcontainers to manage this project. It makes it easier to run/build in a Linux container running on a Mac.
Here are some hints for using devcontainers outside of VSCode:

To start the container:

    devcontainer up --workspace-folder .

To open a bash prompt in the container:

    devcontainer exec --workspace-folder . bash

To stop the container, use the normal docker workflow:

    docker stop $NAME

To rebuild the container's image:

    docker rmi $NAME
