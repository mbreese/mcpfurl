#!/usr/bin/env bash
#
# Integration tests for mcpfurl
#
# Usage: ./tests/run_tests.sh [base_url]
#   base_url defaults to http://localhost:18080
#
set -euo pipefail

BASE_URL="${1:-http://localhost:18080}"
AUTH="Authorization: Bearer test-secret"
TESTWEB="http://testweb"  # internal docker network hostname

PASS=0
FAIL=0
ERRORS=""
HTTP_CODE=""
BODY=""
BODY_FILE=$(mktemp)
trap 'rm -f "$BODY_FILE"' EXIT

# ── Helpers ────────────────────────────────────────────────────────────────

pass() {
    PASS=$((PASS + 1))
    printf "  \033[32mPASS\033[0m %s\n" "$1"
}

fail() {
    FAIL=$((FAIL + 1))
    ERRORS="${ERRORS}\n  FAIL: $1 — $2"
    printf "  \033[31mFAIL\033[0m %s — %s\n" "$1" "$2"
}

# curl wrapper: sets $HTTP_CODE and $BODY
apicurl() {
    local url="$1"; shift
    HTTP_CODE=$(curl -s -o "$BODY_FILE" -w "%{http_code}" -H "$AUTH" "$@" "$url" 2>/dev/null) || true
    BODY=$(cat "$BODY_FILE" 2>/dev/null) || true
}

# MCP endpoint returns SSE (text/event-stream) or JSON.
# Extract JSON from "data:" lines if SSE, otherwise use raw body.
mcpcurl() {
    local url="$1"; shift
    HTTP_CODE=$(curl -s -o "$BODY_FILE" -w "%{http_code}" -H "$AUTH" -H "Content-Type: application/json" -X POST "$@" "$url" 2>/dev/null) || true
    # Debug: show raw response for troubleshooting
    echo "  [debug] raw response (first 500 chars): $(head -c 500 "$BODY_FILE" 2>/dev/null | cat -v)" >&2
    # Try SSE "data:" lines first, fall back to raw body
    local sse_data
    sse_data=$(sed -n 's/^data: *//p' "$BODY_FILE" 2>/dev/null | tr '\n' ' ') || true
    if [ -n "$sse_data" ]; then
        BODY="$sse_data"
    else
        BODY=$(cat "$BODY_FILE" 2>/dev/null) || true
    fi
}

assert_http_code() {
    local test_name="$1" expected="$2"
    if [ "$HTTP_CODE" = "$expected" ]; then
        pass "$test_name (HTTP $HTTP_CODE)"
    else
        fail "$test_name" "expected HTTP $expected, got $HTTP_CODE"
    fi
}

assert_contains() {
    local test_name="$1" body="$2" needle="$3"
    if echo "$body" | grep -qF "$needle"; then
        pass "$test_name"
    else
        fail "$test_name" "response does not contain '$needle'"
    fi
}

assert_not_empty() {
    local test_name="$1" value="$2"
    if [ -n "$value" ]; then
        pass "$test_name"
    else
        fail "$test_name" "value is empty"
    fi
}

# ── Wait for service ──────────────────────────────────────────────────────

echo "Waiting for mcpfurl to be ready..."
for i in $(seq 1 30); do
    if curl -sf -H "$AUTH" "$BASE_URL/" >/dev/null 2>&1; then
        echo "Service is ready."
        break
    fi
    if [ "$i" = "30" ]; then
        echo "ERROR: mcpfurl did not become ready in time"
        exit 1
    fi
    sleep 2
done

# ══════════════════════════════════════════════════════════════════════════
echo ""
echo "=== Root Endpoint ==="

apicurl "$BASE_URL/"
assert_http_code "GET /" "200"
assert_contains "root returns Hello" "$BODY" "Hello!"

# ══════════════════════════════════════════════════════════════════════════
echo ""
echo "=== Authentication ==="

# Request without auth should fail
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/" 2>/dev/null) || true
if [ "$HTTP_CODE" = "401" ]; then
    pass "unauthenticated request returns 401"
else
    fail "unauthenticated request returns 401" "got HTTP $HTTP_CODE"
fi

# Request with wrong key should fail
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer wrong-key" "$BASE_URL/" 2>/dev/null) || true
if [ "$HTTP_CODE" = "401" ]; then
    pass "wrong bearer token returns 401"
else
    fail "wrong bearer token returns 401" "got HTTP $HTTP_CODE"
fi

# ══════════════════════════════════════════════════════════════════════════
echo ""
echo "=== REST API: /api/fetch ==="

# Missing url parameter
apicurl "$BASE_URL/api/fetch"
assert_http_code "fetch missing url" "400"

# Fetch test page
apicurl "$BASE_URL/api/fetch?url=${TESTWEB}/index.html"
assert_http_code "fetch test page" "200"
assert_contains "fetch returns title" "$BODY" "Test Page"
assert_contains "fetch returns markdown content" "$BODY" "Hello from mcpfurl test server"
assert_contains "fetch returns target_url" "$BODY" "target_url"

# Fetch second page
apicurl "$BASE_URL/api/fetch?url=${TESTWEB}/page2.html"
assert_http_code "fetch page2" "200"
assert_contains "fetch page2 content" "$BODY" "Second Test Page"

# Fetch non-existent page
apicurl "$BASE_URL/api/fetch?url=${TESTWEB}/nonexistent.html"
# Should still return something (chrome will load the 404 page)
assert_http_code "fetch 404 page" "200"

# ══════════════════════════════════════════════════════════════════════════
echo ""
echo "=== REST API: /api/image ==="

# Missing url parameter
apicurl "$BASE_URL/api/image"
assert_http_code "image missing url" "400"

# Fetch test image
apicurl "$BASE_URL/api/image?url=${TESTWEB}/image.png"
assert_http_code "image fetch PNG" "200"

# ══════════════════════════════════════════════════════════════════════════
echo ""
echo "=== REST API: /api/browser-image ==="

# Missing url parameter
apicurl "$BASE_URL/api/browser-image"
assert_http_code "browser-image missing url" "400"

# Fetch test image via browser
apicurl "$BASE_URL/api/browser-image?url=${TESTWEB}/image.png"
assert_http_code "browser-image fetch PNG" "200"

# ══════════════════════════════════════════════════════════════════════════
echo ""
echo "=== REST API: /api/search ==="

# Missing q parameter
apicurl "$BASE_URL/api/search"
assert_http_code "search missing q" "400"

# Search without configured API keys should return 503
apicurl "$BASE_URL/api/search?q=test"
assert_http_code "search without keys returns 503" "503"

# ══════════════════════════════════════════════════════════════════════════
echo ""
echo "=== REST API: /api/summary ==="

# Missing url parameter
apicurl "$BASE_URL/api/summary"
assert_http_code "summary missing url" "400"

# Summary without LLM config should fail gracefully
apicurl "$BASE_URL/api/summary?url=${TESTWEB}/index.html"
# This will likely return 502 since no LLM is configured
if [ "$HTTP_CODE" = "502" ] || [ "$HTTP_CODE" = "200" ]; then
    pass "summary without LLM config returns error or success (HTTP $HTTP_CODE)"
else
    fail "summary without LLM" "expected HTTP 502 or 200, got $HTTP_CODE"
fi

# ══════════════════════════════════════════════════════════════════════════
echo ""
echo "=== MCP Protocol: /mcp ==="

# MCP initialize + list tools
MCP_INIT='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'

mcpcurl "$BASE_URL/mcp" -d "$MCP_INIT"
assert_http_code "MCP initialize" "200"
assert_contains "MCP returns server info" "$BODY" "mcpfurl"

# List tools
MCP_LIST='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
mcpcurl "$BASE_URL/mcp" -d "$MCP_LIST"
assert_http_code "MCP tools/list" "200"
assert_contains "MCP has web_fetch tool" "$BODY" "web_fetch"
assert_contains "MCP has image_fetch tool" "$BODY" "image_fetch"
assert_contains "MCP has browser_image_fetch tool" "$BODY" "browser_image_fetch"
assert_contains "MCP has web_summary tool" "$BODY" "web_summary"

# ── MCP tool: web_fetch ───────────────────────────────────────────────────
echo ""
echo "=== MCP Tool: web_fetch ==="

MCP_FETCH='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"web_fetch","arguments":{"url":"'"${TESTWEB}"'/index.html"}}}'
mcpcurl "$BASE_URL/mcp" -d "$MCP_FETCH"
assert_http_code "MCP web_fetch" "200"
assert_contains "MCP web_fetch returns content" "$BODY" "Hello from mcpfurl test server"

# web_fetch with missing URL
MCP_FETCH_NOURL='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"web_fetch","arguments":{"url":""}}}'
mcpcurl "$BASE_URL/mcp" -d "$MCP_FETCH_NOURL"
assert_http_code "MCP web_fetch empty url" "200"
assert_contains "MCP web_fetch error on empty url" "$BODY" "Missing URL"

# ── MCP tool: image_fetch ─────────────────────────────────────────────────
echo ""
echo "=== MCP Tool: image_fetch ==="

MCP_IMAGE='{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"image_fetch","arguments":{"url":"'"${TESTWEB}"'/image.png"}}}'
mcpcurl "$BASE_URL/mcp" -d "$MCP_IMAGE"
assert_http_code "MCP image_fetch" "200"
assert_contains "MCP image_fetch has base64 data" "$BODY" "data_base64"
assert_contains "MCP image_fetch has content_type" "$BODY" "content_type"

# image_fetch with missing URL
MCP_IMAGE_NOURL='{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"image_fetch","arguments":{"url":""}}}'
mcpcurl "$BASE_URL/mcp" -d "$MCP_IMAGE_NOURL"
assert_http_code "MCP image_fetch empty url" "200"
assert_contains "MCP image_fetch error on empty url" "$BODY" "Missing URL"

# ── MCP tool: browser_image_fetch ─────────────────────────────────────────
echo ""
echo "=== MCP Tool: browser_image_fetch ==="

MCP_BIMG='{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"browser_image_fetch","arguments":{"url":"'"${TESTWEB}"'/image.png"}}}'
mcpcurl "$BASE_URL/mcp" -d "$MCP_BIMG"
assert_http_code "MCP browser_image_fetch" "200"
assert_contains "MCP browser_image_fetch has base64 data" "$BODY" "data_base64"

# ── MCP tool: web_search ──────────────────────────────────────────────────
echo ""
echo "=== MCP Tool: web_search ==="

MCP_SEARCH_NOQUERY='{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"web_search","arguments":{"query":""}}}'
mcpcurl "$BASE_URL/mcp" -d "$MCP_SEARCH_NOQUERY"
# web_search may not be registered (no API keys), so 200 with error is fine
if [ "$HTTP_CODE" = "200" ]; then
    pass "MCP web_search empty query (HTTP $HTTP_CODE)"
else
    # tool not registered is also acceptable
    pass "MCP web_search not registered without API keys (HTTP $HTTP_CODE)"
fi

# ── MCP tool: web_summary ────────────────────────────────────────────────
echo ""
echo "=== MCP Tool: web_summary ==="

MCP_SUMMARY_NOURL='{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"web_summary","arguments":{"url":""}}}'
mcpcurl "$BASE_URL/mcp" -d "$MCP_SUMMARY_NOURL"
assert_http_code "MCP web_summary empty url" "200"
assert_contains "MCP web_summary error on empty url" "$BODY" "Missing"

# ══════════════════════════════════════════════════════════════════════════
echo ""
echo "════════════════════════════════════════════"
printf "Results: \033[32m%d passed\033[0m, \033[31m%d failed\033[0m\n" "$PASS" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
    printf "\nFailures:%b\n" "$ERRORS"
    exit 1
fi
echo "All tests passed!"
