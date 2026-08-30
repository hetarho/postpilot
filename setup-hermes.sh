#!/bin/sh
set -eu

REPO_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
INSTALLER="$REPO_ROOT/agent/packaging/install.sh"
AGENT_ROOT="$HOME/Library/Application Support/Postpilot Agent"
AGENT_BIN="$AGENT_ROOT/bin/postpilot-agent"
AGENT_CONFIG="$AGENT_ROOT/config.json"
OPEN_SETUP=false

case "${1:-}" in
  "") ;;
  --setup) OPEN_SETUP=true ;;
  *)
    printf '%s\n' "사용법: ./setup-hermes.sh [--setup]" >&2
    exit 2
    ;;
esac

printf '%s\n' "Postpilot Mac agent를 설치하거나 업데이트합니다..."
"$INSTALLER"

has_armed_connection() {
  [ -f "$AGENT_CONFIG" ] && grep -Eq '"armed"[[:space:]]*:[[:space:]]*true' "$AGENT_CONFIG"
}

if [ "$OPEN_SETUP" = true ] || ! has_armed_connection; then
  if pgrep -f "$AGENT_BIN setup" >/dev/null 2>&1; then
    printf '%s\n' "Postpilot 연결 화면이 이미 실행 중입니다. 기존 화면에서 연결을 마친 뒤 다시 실행해 주세요." >&2
    exit 1
  fi
  printf '%s\n' "브라우저에서 Postpilot 연결을 완료해 주세요. 완료되면 이 단계는 자동으로 끝납니다."
  "$AGENT_BIN" setup
fi

if ! has_armed_connection; then
  printf '%s\n' "활성 연결이 없어 자동 실행을 설치하지 않았습니다. 다시 실행해 연결을 완료해 주세요." >&2
  exit 1
fi

printf '%s\n' "연결 상태를 확인합니다..."
"$AGENT_BIN" diagnostics

printf '%s\n' "로그인 시 자동 실행을 등록합니다..."
"$AGENT_BIN" install

printf '%s\n' "완료: Postpilot Mac agent가 백그라운드에서 자동으로 대기합니다."
