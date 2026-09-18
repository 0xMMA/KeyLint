#!/usr/bin/env bash
set -euo pipefail

# Gets the Microsoft WebView2 bootstrapper and refuses to hand over a file that
# is not signed by Microsoft.
#
# The bootstrapper is an executable bundled into the installer end users run,
# which makes it the one external binary in this pipeline that reaches other
# people's machines (#66). Before this it was fetched with a bare `curl` and no
# check of any kind.
#
#   ./scripts/fetch-webview2.sh [output-path]      # download, verify, install
#   ./scripts/fetch-webview2.sh --verify-only PATH # check a file already there
#
# Default output is build/windows/nsis/MicrosoftEdgeWebview2Setup.exe.

# Overridable so the redirect guard can be tested against a host that is not
# Microsoft's. It cannot weaken anything: whatever the URL, the file still has
# to carry a valid Microsoft signature.
LINK="${WEBVIEW2_LINK:-https://go.microsoft.com/fwlink/p/?LinkId=2124703}"

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

readonly MS_ROOT="build/windows/microsoft-root-ca-2011.pem"

# hostOf extracts the host from a URL, defensively.
#
# Userinfo has to be stripped BEFORE the port, or a redirect to
# `https://microsoft.com:443@evil.example.com/x` parses as "microsoft.com" —
# curl really does carry that through to url_effective, so this is a live
# bypass and not a hypothetical one.
hostOf() {
    local host=${1#*://}
    host=${host%%[/?#]*}   # path, query, fragment
    host=${host##*@}       # userinfo
    host=${host%%:*}       # port
    echo "${host,,}"       # a host in capitals is still that host
}

# verifyFile is the gate: the file must carry an Authenticode signature that
# chains to Microsoft's code-signing root.
#
# -index 0 selects the primary signature. These files carry a second, self-signed
# "EdgeBuild" certificate that cannot chain anywhere, and without -index a
# perfectly genuine file fails verification because of it.
verifyFile() {
    local file=$1
    if ! command -v osslsigncode >/dev/null; then
        echo "osslsigncode is not installed — cannot verify $file" >&2
        echo "  Ubuntu/Debian: sudo apt-get install osslsigncode" >&2
        return 1
    fi
    [[ -f "$MS_ROOT" ]] || { echo "Missing $MS_ROOT — see .claude/docs/versioning.md" >&2; return 1; }

    local out
    if ! out=$(osslsigncode verify -index 0 -CAfile "$MS_ROOT" -in "$file" 2>&1); then
        echo "Refusing: $file is not signed by Microsoft." >&2
        echo "$out" | grep -E "Subject:|MISMATCH|Error|Failed" | sed 's/^/    /' >&2
        return 1
    fi
    echo "  signed by:$(echo "$out" | grep -m1 -oE 'CN=[^,/]*')"
}

# --- arguments -------------------------------------------------------------

if [[ "${1:-}" == "--verify-only" ]]; then
    [[ -n "${2:-}" ]] || { echo "--verify-only needs a path" >&2; exit 2; }
    verifyFile "$2"
    echo "  $2 verified"
    exit 0
fi

OUT="${1:-build/windows/nsis/MicrosoftEdgeWebview2Setup.exe}"

# Already there and genuine? Keep it. This is what lets an offline build work,
# and it is strictly better than the old behaviour, which trusted an existing
# file without looking at it.
if [[ -f "$OUT" ]] && verifyFile "$OUT" >/dev/null 2>&1; then
    echo "WebView2 bootstrapper already present and signed by Microsoft — keeping it."
    exit 0
fi

# --- download --------------------------------------------------------------

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT
TMP="$TMP_DIR/MicrosoftEdgeWebview2Setup.exe"

echo "Fetching the WebView2 bootstrapper…"
# --proto '=https' refuses to be redirected onto a plaintext scheme. The
# redirector's target is a CDN host, so -L is required and the destination is
# checked below rather than trusted.
FINAL_URL=$(curl --proto '=https' --proto-redir '=https' -fsSL \
    --max-time 300 --retry 3 --retry-delay 5 \
    -o "$TMP" -w '%{url_effective}' "$LINK")

HOST=$(hostOf "$FINAL_URL")
if [[ "$HOST" != "microsoft.com" && "$HOST" != *.microsoft.com ]]; then
    echo "Refusing: the redirector sent us to $HOST, which is outside microsoft.com" >&2
    echo "  final URL: $FINAL_URL" >&2
    exit 1
fi
echo "  served by $HOST"

verifyFile "$TMP"

mkdir -p "$(dirname "$OUT")"
mv "$TMP" "$OUT"
echo "  → $OUT"
