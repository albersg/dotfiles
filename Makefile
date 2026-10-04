# Local preflight: the CI checks that can run on this machine, in CI's order,
# before a CI cycle is spent on them. Each step prints the CI job it mirrors and
# the script names the jobs it cannot run. See scripts/preflight.sh.
.PHONY: preflight

preflight:
	bash scripts/preflight.sh
