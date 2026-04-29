FROM chromedp/headless-shell:stable AS chrome

FROM debian:bookworm-slim

LABEL org.opencontainers.image.source=https://github.com/mbreese/mcpfurl
LABEL org.opencontainers.image.description="MCP Server for fetching web pages, images, or performing Google searches"

WORKDIR /app

# Copy headless Chrome from the chromedp image instead of using Debian's
# Chromium package, which crashes in K8s containers (Chromium 147 SIGTRAP).
COPY --from=chrome /headless-shell /headless-shell
RUN apt update && \
    apt -y upgrade && \
    apt install -y sqlite3 curl fontconfig libnss3 libatk1.0-0 \
        libatk-bridge2.0-0 libcups2 libxdamage1 libpango-1.0-0 \
        libcairo2 libasound2 libxrandr2 libxcomposite1 libxshmfence1 \
        libgbm1 && \
    mkdir -p /app /var/cache/fontconfig && \
    chmod 777 /var/cache/fontconfig && \
    fc-cache -f && \
    useradd -d /app -s /bin/bash user

COPY bin/mcpfurl.linux_musl_amd64 /app/mcpfurl
RUN chmod +x /app/mcpfurl

USER user

CMD /app/mcpfurl mcp-http
