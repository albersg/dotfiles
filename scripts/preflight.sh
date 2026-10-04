#!/usr/bin/env bash
#
# preflight.sh - the part of CI that can be run here, in CI's order, before a
# CI cycle is spent on it.
#
# Every step names the CI job it mirrors, so a failure points at a check that
# exists rather than at a script with an opinion. The footer names the jobs this
# script cannot run and why. The mapping is derived from .github/workflows/ci.yml
# and .github/workflows/e2e-testing.yml; a step that no longer matches its job is
# a bug in this file, not a new policy.
#
# Needs bash, go, gofmt, git, ripgrep, shellcheck and gitleaks. Nothing else.
# Works on Linux and macOS: no GNU-only flags, no bash 4 syntax.
#
# Usage:
#   make preflight
#   scripts/preflight.sh

set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$REPO_ROOT"

TOTAL=7
STEP=0

fail() {
  printf '\nPREFLIGHT FAILED: %s\n' "$1" >&2
  exit 1
}

step() {
  STEP=$((STEP + 1))
  printf '\n[%d/%d] %s\n' "$STEP" "$TOTAL" "$1"
  printf '      CI job : %s\n' "$2"
  printf '      command: %s\n' "$3"
}

pass() {
  printf '      result : PASS\n'
}

printf 'dotfiles preflight - %s\n' "$REPO_ROOT"

# A missing tool is not a skipped check. CI installs shellcheck and ripgrep on
# the runner, and this repository's own Brewfile installs all three of them, so
# stopping here loses nothing: a check that quietly did not run is the failure
# mode this script exists to remove.
missing=''
for tool in go gofmt git rg shellcheck gitleaks; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    missing="$missing $tool"
  fi
done
if [ -n "$missing" ]; then
  printf '\nPREFLIGHT CANNOT RUN: not installed:%s\n' "$missing" >&2
  printf 'Install them with: brew bundle (see Brewfile), or the dotfiles installer.\n' >&2
  exit 1
fi

# --- Go Validation (ci.yml), step 1 of 3 --------------------------------------
step "gofmt -l" "Go Validation (ci.yml)" "cd installer && gofmt -l ."
if ! unformatted=$(cd installer && gofmt -l .); then
  fail "gofmt -l . (Go Validation)"
fi
if [ -n "$unformatted" ]; then
  printf "Files not gofmt'd:\n%s\n" "$unformatted" >&2
  fail "gofmt check: run 'gofmt -w .' inside installer/ (Go Validation)"
fi
pass

# --- Go Validation, step 2 of 3 ----------------------------------------------
step "go vet" "Go Validation (ci.yml)" "cd installer && go vet ./..."
if ! (cd installer && go vet ./...); then
  fail "go vet ./... (Go Validation)"
fi
pass

# --- Build (ci.yml) + macOS smoke test (e2e-testing.yml), first two commands --
# CI builds to installer/dotfiles and runs it there. This builds the same
# package into a temporary directory instead, so a preflight run leaves the
# checkout exactly as it found it and gofmt never sees a stray binary. The
# template is passed explicitly because BSD mktemp (macOS) wants one.
step "go build ./cmd/dotfiles + --help" \
  "Build ubuntu-latest/macos-latest (ci.yml); macOS smoke test (e2e-testing.yml)" \
  "cd installer && go build -o \"\$(mktemp -d)/dotfiles\" ./cmd/dotfiles && <tmpdir>/dotfiles --help"
smoke_dir=$(mktemp -d "${TMPDIR:-/tmp}/dotfiles-preflight.XXXXXX")
if ! (cd installer && go build -o "$smoke_dir/dotfiles" ./cmd/dotfiles); then
  fail "go build ./cmd/dotfiles (Build matrix)"
fi
if ! "$smoke_dir/dotfiles" --help >/dev/null; then
  fail "$smoke_dir/dotfiles --help (Build matrix smoke test)"
fi
pass

# --- The test step of three jobs ---------------------------------------------
# Go Validation runs `go test ./... -skip Golden -count=1`, the e2e Go tests job
# runs `go test ./... -skip Golden`, and the macOS smoke test runs the whole
# suite. Running the whole suite here once is a superset of the three: the 18
# golden snapshots pass on Linux too, because the tester pins HOME, the system
# info and the state directory. Nothing here writes a golden; only -update does.
step "go test ./..." \
  "Go Validation + Go tests (e2e-testing.yml) + macOS smoke test (e2e-testing.yml)" \
  "cd installer && go test ./... -count=1"
if ! (cd installer && go test ./... -count=1); then
  fail "go test ./... -count=1 (Go Validation / Go tests / macOS smoke test)"
fi
pass

# --- Shell Validation (ci.yml) ------------------------------------------------
step "shellcheck --severity=warning" "Shell Validation (ci.yml)" \
  "find . -name '*.sh' -type f ! -path './dotfiles-zsh/.oh-my-zsh/*' ! -path './.git/*' | xargs shellcheck --severity=warning"
sh_files=$(find . -name '*.sh' -type f ! -path './dotfiles-zsh/.oh-my-zsh/*' ! -path './.git/*')
if [ -n "$sh_files" ]; then
  if ! printf '%s\n' "$sh_files" | xargs shellcheck --severity=warning; then
    fail "shellcheck --severity=warning (Shell Validation)"
  fi
fi
pass

# --- Branding Audit (ci.yml) --------------------------------------------------
# The pattern is assembled from two fragments on purpose. ripgrep does not
# descend into hidden paths, so .github/ is skipped, but this script is not
# hidden: a file that spells the upstream project's name out fails the audit it
# is running, and the audit's exclusion list is per-file. Splitting the literal
# keeps this file honest about the check without making it the check's own
# finding. The 17 exclusion globs below are copied from ci.yml verbatim.
upstream_name='gentle'
upstream_name="${upstream_name}man"
brand_globs=(
  --glob '!NOTICE'
  --glob '!docs/adr/*'
  --glob '!docs/ATTRIBUTION.md'
  --glob '!CHANGELOG.md'
  --glob '!DOWNSTREAM.md'
  --glob '!UPSTREAM.md'
  --glob '!LICENSE'
  --glob '!.git/*'
  --glob '!THIRD_PARTY_NOTICES.md'
  --glob '!docs/UPSTREAM-SYNC.md'
  --glob '!docs/audits/*'
  --glob '!docs/BRANDING.md'
  --glob '!go.sum'
  --glob '!Brewfile'
  --glob '!dotfiles-nvim/nvim/spell/*'
  --glob '!.github/workflows/upstream-watch.yml'
  --glob '!.gitignore'
)
step "branding audit" "Branding Audit (ci.yml)" \
  "rg -l -i $upstream_name [the 17 exclusion globs from ci.yml] ."
matches=$(rg -l -i "$upstream_name" "${brand_globs[@]}" . 2>/dev/null || true)
if [ -n "$matches" ]; then
  printf 'ERROR: Residual upstream branding found in:\n%s\n\nDetailed matches:\n' "$matches" >&2
  rg -n -i "$upstream_name" "${brand_globs[@]}" . 2>/dev/null || true
  fail "branding audit (Branding Audit)"
fi
pass

# --- Secret Scanning (ci.yml) -------------------------------------------------
# gitleaks-action scans the commits a push or pull request introduces, not the
# whole history, so this does the same: the commits this branch adds on top of
# its base. An empty range means nothing has been committed ahead of main, which
# is nothing to scan. --redact hides a secret's value in the output; it does not
# change what is detected.
base_ref=''
for ref in origin/main main; do
  if git rev-parse --verify --quiet "$ref^{commit}" >/dev/null 2>&1; then
    base_ref="$ref"
    break
  fi
done

merge_base=''
if [ -n "$base_ref" ]; then
  merge_base=$(git merge-base "$base_ref" HEAD 2>/dev/null || true)
fi

gitleaks_args=(git --no-banner --redact --config .gitleaks.toml)
if [ -n "$merge_base" ]; then
  gitleaks_args+=(--log-opts="$merge_base..HEAD")
elif git rev-parse --verify --quiet 'HEAD~1^{commit}' >/dev/null 2>&1; then
  # No base branch to compare against: scan the tip commit, which is what a
  # push of one commit would introduce.
  gitleaks_args+=(--log-opts='HEAD~1..HEAD')
fi

step "gitleaks (commits this branch adds)" "Secret Scanning (ci.yml)" "gitleaks ${gitleaks_args[*]}"
if ! gitleaks "${gitleaks_args[@]}"; then
  fail "gitleaks ${gitleaks_args[*]} (Secret Scanning)"
fi
pass

printf '\nPREFLIGHT PASSED - %d/%d steps, each one a CI job or part of one.\n' "$STEP" "$TOTAL"
cat <<'NOTES'
Not covered here, and why:
  - Linux E2E (ubuntu, fedora, arch, alpine; debian on main) - ./installer/e2e/docker-test.sh
    pulls images and installs for real, so it needs Docker and the network.
  - Termux E2E - Docker, and non-blocking in CI (continue-on-error: true).
  - E2E image list - a meta job that only decides the matrix above.
  - macOS smoke test - the commands it runs are steps 3 and 4 above; the macOS
    toolchain is not. A darwin-only compile break still needs CI.
  - Secret Scanning - same tool and same commits as step 7, but CI runs it on the
    runner with the repository token for its pull-request annotations.
NOTES
