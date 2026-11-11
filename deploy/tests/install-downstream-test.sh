#!/bin/bash
set -euo pipefail

TEST_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
TEST_TEMP=$(mktemp -d)
trap 'rm -rf "$TEST_TEMP"' EXIT

mkdir -p "$TEST_TEMP/bin" "$TEST_TEMP/install"
cat > "$TEST_TEMP/bin/curl" <<'EOF'
#!/bin/bash
printf '%s\n' "$@" > "$TEST_CURL_LOG"
if [[ "$*" == *'/releases/latest'* ]]; then
    printf '{"tag_name":"%s"}\n' "${TEST_LATEST_TAG:-v0.2.21-zy.2}"
else
    printf '200'
fi
EOF
cat > "$TEST_TEMP/install/sub2api" <<'EOF'
#!/bin/sh
printf 'Sub2API 0.2.21-zy.10 (commit: fixture)\n'
EOF
chmod +x "$TEST_TEMP/bin/curl" "$TEST_TEMP/install/sub2api"

TEST_CURL_LOG="$TEST_TEMP/curl.log" PATH="$TEST_TEMP/bin:$PATH" UPDATE_GITHUB_TOKEN= \
    bash -c '
      source <(head -n -1 "$1")
      INSTALL_DIR="$2"
      test "$GITHUB_REPO" = zhiylee/sub2api
      test "$(get_current_version)" = 0.2.21-zy.10
      test "$(validate_version 0.2.21-zy.2)" = v0.2.21-zy.2
      get_latest_version >/dev/null
      test "$LATEST_VERSION" = v0.2.21-zy.2
    ' bash "$TEST_ROOT/deploy/install.sh" "$TEST_TEMP/install"
grep -Fxq 'https://api.github.com/repos/zhiylee/sub2api/releases/latest' "$TEST_TEMP/curl.log"

for version in v0.2.21 v0.2.21-rc.1 v0.2.21-zy.0 v00.2.21-zy.1; do
    if bash -c 'source <(head -n -1 "$1"); validate_version "$2"' bash \
        "$TEST_ROOT/deploy/install.sh" "$version" >/dev/null 2>&1; then
        echo "installer accepted a non-distribution version: $version" >&2
        exit 1
    fi
done
if TEST_CURL_LOG="$TEST_TEMP/curl.log" TEST_LATEST_TAG=v0.2.22 PATH="$TEST_TEMP/bin:$PATH" UPDATE_GITHUB_TOKEN= \
    bash -c 'source <(head -n -1 "$1"); get_latest_version' bash "$TEST_ROOT/deploy/install.sh" >/dev/null 2>&1; then
    echo 'installer accepted a bare official version as the latest fork release' >&2
    exit 1
fi
echo 'downstream installer checks passed'
