# Local checks in two speeds. `make check` is the inner loop: format, vet and the
# tests for the packages this branch changes, with Go's test cache left on, so it
# is fast enough to run after every edit. `make preflight` is the full gate: the
# CI checks that can run on this machine, in CI's order, run once before pushing.
# Both live in scripts/preflight.sh; run `make check` often and `make preflight`
# once.
.PHONY: check preflight

check:
	bash scripts/preflight.sh --check

preflight:
	bash scripts/preflight.sh
