#!/bin/sh
# Host-side runner for the one-password probe.
#
# The Go tests prove the generated scripts carry `sudo -n` and no keypress
# prompt. This runs the real generated scripts under a real sudo in a container
# and counts how many times sudo asks for the password, so the claim PR #304
# makes ("the whole install asks for the password once, and only once") is
# measured instead of asserted about strings.
#
# Usage:
#   ./sudo-prompt-test.sh                         # assert the working tree asks once
#   ./sudo-prompt-test.sh --ref <commit>          # assert another revision asks once
#   ./sudo-prompt-test.sh --measure --ref <commit># print the numbers, assert nothing
#   ./sudo-prompt-test.sh --expect-total 1 --expect-steps 0
#
# The scripts are rendered by the Go test TestRenderInstallScriptsForE2E from
# the revision's own getInteractiveScript, so the bytes measured are the bytes
# the installer would run. Nothing but the container ever executes sudo: the
# host only builds an image, renders files and reads the container's output.

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALLER_DIR="$(dirname "$SCRIPT_DIR")"
REPO_ROOT="$(dirname "$INSTALLER_DIR")"
RENDERER="$INSTALLER_DIR/internal/tui/install_script_render_test.go"

IMAGE_TAG="dotfiles-sudo-prompt"
REF=""
EXPECT_TOTAL="1"
EXPECT_STEPS="0"
KEEP_TEMP="${KEEP_TEMP:-0}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# strip_ansi removes SGR colour sequences (reads stdin, writes stdout), matching
# docker-test.sh: the probe prints its own colours, so anything parsing them has
# to strip them first.
strip_ansi() {
    sed "s/$(printf '\033')\[[0-9;]*m//g"
}

print_header() {
    echo ""
    echo "${CYAN}╔══════════════════════════════════════════════════════════╗${NC}"
    echo "${CYAN}║${NC}  ${BOLD}$1${NC}"
    echo "${CYAN}╚══════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

usage() {
    cat <<'USAGE'
Host-side runner for the one-password probe.

Usage:
  ./sudo-prompt-test.sh                          assert the working tree asks once
  ./sudo-prompt-test.sh --ref <commit>           assert another revision asks once
  ./sudo-prompt-test.sh --measure --ref <commit> print the numbers, assert nothing
  ./sudo-prompt-test.sh --expect-total N --expect-steps N
  KEEP_TEMP=1 ./sudo-prompt-test.sh              keep the rendered scripts

Run `go test ./internal/tui/... -timeout 25m -count=1` for the unit tests;
this script is the container test that measures a real sudo.
USAGE
}

while [ $# -gt 0 ]; do
    case "$1" in
        --ref) REF="$2"; shift 2 ;;
        --expect-total) EXPECT_TOTAL="$2"; shift 2 ;;
        --expect-steps) EXPECT_STEPS="$2"; shift 2 ;;
        --measure) EXPECT_TOTAL=""; EXPECT_STEPS=""; shift ;;
        --keep-temp) KEEP_TEMP=1; shift ;;
        -h|--help) usage; exit 0 ;;
        *) echo "${RED}Unknown option: $1${NC}" >&2; usage >&2; exit 2 ;;
    esac
done

render_dir="$(mktemp -d /tmp/dotfiles-sudo-prompts.XXXXXX)"
source_tree=""
render_root="$INSTALLER_DIR"

cleanup() {
    if [ "$KEEP_TEMP" = "1" ]; then
        echo "${YELLOW}kept: $render_dir${source_tree:+ $source_tree}${NC}"
        return
    fi
    # Only the directories this run created, named from mktemp, are removed.
    # `return 0` keeps a short-circuited test from becoming the script's exit
    # status through the EXIT trap.
    [ -n "$render_dir" ] && rm -rf "${render_dir:?}"
    [ -n "$source_tree" ] && rm -rf "${source_tree:?}"
    return 0
}
trap cleanup EXIT INT TERM

# A revision is rendered from its own tree: the renderer test is dropped into an
# archive of that commit, so it calls that commit's getInteractiveScript and the
# measured scripts cannot come from the working tree by accident.
if [ -n "$REF" ]; then
    source_tree="$(mktemp -d /tmp/dotfiles-sudo-source.XXXXXX)"
    git -C "$REPO_ROOT" archive "$REF" | tar -x -C "$source_tree"
    cp "$RENDERER" "$source_tree/installer/internal/tui/"
    render_root="$source_tree/installer"
fi

print_header "One password, one prompt"
if [ -n "$REF" ]; then
    commit="$(git -C "$REPO_ROOT" rev-parse --short "$REF")"
    echo "${BLUE}→ Rendering step scripts from ${BOLD}$commit${NC}${BLUE} (${REF})${NC}"
else
    echo "${BLUE}→ Rendering step scripts from the working tree${NC}"
fi

if ! ( cd "$render_root" && DOTFILES_E2E_RENDER_DIR="$render_dir" go test ./internal/tui/ -run TestRenderInstallScriptsForE2E -count=1 ); then
    echo "${RED}✗ Could not render the step scripts${NC}"
    exit 1
fi
echo "${GREEN}✓ Rendered $(wc -l < "$render_dir/manifest.txt" | tr -d ' ') step scripts${NC}"

echo "${BLUE}→ Building ${IMAGE_TAG}...${NC}"
if ! docker build -f "$SCRIPT_DIR/Dockerfile.sudoprompt" -t "$IMAGE_TAG" "$SCRIPT_DIR" >/dev/null; then
    echo "${RED}✗ Could not build $IMAGE_TAG${NC}"
    exit 1
fi

expect_env=""
if [ -n "$EXPECT_TOTAL" ]; then
    expect_env="-e EXPECT_TOTAL=$EXPECT_TOTAL"
fi
if [ -n "$EXPECT_STEPS" ]; then
    expect_env="$expect_env -e EXPECT_STEPS=$EXPECT_STEPS"
fi

echo "${BLUE}→ Running the probe...${NC}"
set +e
# shellcheck disable=SC2086
output="$(docker run --rm $expect_env -v "$render_dir:/opt/step-scripts:ro" "$IMAGE_TAG" 2>&1)"
status=$?
set -e

printf '%s\n' "$output" | strip_ansi

echo ""
echo "${BOLD}Measured${NC}"
printf '%s\n' "$output" | strip_ansi | grep '^SUDO_' || true

if [ "$status" -eq 0 ]; then
    if [ -n "$EXPECT_TOTAL" ]; then
        echo "${GREEN}✓ Whole run asked $EXPECT_TOTAL time(s); step scripts asked $EXPECT_STEPS${NC}"
    else
        echo "${GREEN}✓ Probe finished (measurement only, nothing asserted)${NC}"
    fi
else
    echo "${RED}✗ The one-password property does not hold${NC}"
fi

exit "$status"
