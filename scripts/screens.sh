#!/usr/bin/env bash
#
# screens.sh - capture the installer's own screens from a real terminal.
#
# The text-frame tests prove where every row lands; they cannot show what the
# interface looks like: the palette, the weight and the space it leaves. This
# script builds the binary and drives it through VHS
# (https://github.com/charmbracelet/vhs), writing PNG stills and a GIF per
# terminal size under captures/. It is opt-in: it is not a CI check and
# preflight never runs it.
#
# Requirements: vhs + ttyd + ffmpeg on PATH, or Docker (--docker; the VHS image
# carries all three).
#
# Usage:
#   scripts/screens.sh                   both sizes, local vhs
#   scripts/screens.sh --docker          same, inside the VHS container
#   scripts/screens.sh vhs/screens-wide.tape
#
# The captures are written to captures/<columns>x<rows>/ and that directory is
# ignored by git. Run it on the machine whose terminal you want to see: the
# tapes use the real home, so the facts on screen are that machine's facts.

set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$REPO_ROOT"

use_docker=false
tapes=()

while [ $# -gt 0 ]; do
  case "$1" in
    --docker) use_docker=true ;;
    -h|--help)
      sed -n '2,23p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    -*) printf 'screens: unknown option %s\n' "$1" >&2; exit 2 ;;
    *) tapes+=("$1") ;;
  esac
  shift
done

if [ "${#tapes[@]}" -eq 0 ]; then
  tapes=(vhs/screens-wide.tape vhs/screens-floor.tape)
fi

# The binary the tapes run. installer/dotfiles is already ignored by git.
if [ "$use_docker" = true ]; then
  command -v docker >/dev/null 2>&1 || {
    printf 'screens: docker is not on PATH\n' >&2
    exit 1
  }
  # The container runs a Linux binary, so build one for the daemon's
  # architecture even on a macOS host, where a plain `go build` is a Mach-O
  # binary the container cannot start.
  arch=$(docker version --format '{{.Server.Arch}}' 2>/dev/null || true)
  if [ -z "$arch" ]; then
    arch=amd64
  fi
  printf 'building installer for linux/%s\n' "$arch"
  ( cd installer && GOOS=linux GOARCH="$arch" go build -o dotfiles ./cmd/dotfiles )
else
  for prog in vhs ttyd ffmpeg; do
    command -v "$prog" >/dev/null 2>&1 || {
      printf 'screens: %s is not on PATH (install vhs, or run with --docker)\n' "$prog" >&2
      exit 1
    }
  done
  printf 'building installer\n'
  ( cd installer && go build -o dotfiles ./cmd/dotfiles )
fi

mkdir -p captures/200x55 captures/80x24

for tape in "${tapes[@]}"; do
  if [ ! -f "$tape" ]; then
    printf 'screens: no such tape: %s\n' "$tape" >&2
    exit 1
  fi
  printf 'recording %s\n' "$tape"
  if [ "$use_docker" = true ]; then
    VHS_NO_SANDBOX=true docker run --rm \
      -w /vhs -v "$REPO_ROOT:/vhs" \
      ghcr.io/charmbracelet/vhs "$tape"
  else
    VHS_NO_SANDBOX=true vhs "$tape"
  fi
done

printf '\ncaptures written under %s/captures:\n' "$REPO_ROOT"
find captures \( -name '*.png' -o -name '*.gif' \) | sort
