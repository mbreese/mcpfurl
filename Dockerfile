FROM debian:stable-slim

LABEL org.opencontainers.image.source=https://github.com/mbreese/mcpfurl
LABEL org.opencontainers.image.description="MCP Server for fetching web pages, images, or performing Google searches"

EXPOSE 8080

WORKDIR /app
RUN apt update && \
    apt -y upgrade && \
    apt install -y chromium-driver sqlite3 && \
    mkdir -p /app && \
    useradd user

COPY bin/mcpfurl.linux_musl_amd64 /app/mcpfurl
RUN chmod +x /app/mcpfurl

USER user

CMD /app/mcpfurl mcp-http
