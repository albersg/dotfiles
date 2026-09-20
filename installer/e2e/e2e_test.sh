#!/bin/sh
# E2E Test Script for dotfiles Installer
# Runs REAL installation tests in Docker containers
#
# Usage: Called by Dockerfiles, not directly

set -e

PASSED=0
FAILED=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_test() {
    printf "${YELLOW}[TEST]${NC} %s\n" "$1"
}

log_pass() {
    printf "${GREEN}[PASS]${NC} %s\n" "$1"
    PASSED=$((PASSED + 1))
}

log_fail() {
    printf "${RED}[FAIL]${NC} %s\n" "$1"
    FAILED=$((FAILED + 1))
}

log_section() {
    printf "\n${YELLOW}════════════════════════════════════════${NC}\n"
    printf "${YELLOW}  %s${NC}\n" "$1"
    printf "${YELLOW}════════════════════════════════════════${NC}\n\n"
}

# ============================================
# PLATFORM - which window manager this run exercises
# ============================================

# E2E_WM is the window manager this platform's run asks the installer for.
#
# Fedora has no route to zellij: the Fedora repositories do not carry it (the
# installer verified that with `dnf install --assumeno`, which is why zellij is
# absent from every Fedora package list in internal/tui/installer.go) and this
# image installs no Homebrew, which is the only other route the installer
# offers. The installer therefore fails the step instead of reporting an install
# that never happened, so asking Fedora for zellij fails the run by design.
# Fedora does carry tmux, so the Fedora run selects tmux; every other platform
# keeps zellij.
#
# Detection mirrors internal/system/detect.go, whose isFedora() stats
# /etc/fedora-release.
E2E_WM="zellij"
if [ -f /etc/fedora-release ]; then
    E2E_WM="tmux"
fi

# E2E_WM_OTHER is the window manager this run does not select, so the shell
# configuration can be checked for it: WM_CMD holds one window manager, and a
# leftover from another choice means the patch never ran.
if [ "$E2E_WM" = "tmux" ]; then
    E2E_WM_OTHER="zellij"
else
    E2E_WM_OTHER="tmux"
fi

# E2E_REQUESTED_WMS lists every window manager this suite asks the installer to
# install on this platform. test_wm_installed asserts these and only these: a
# window manager that was requested and is absent is a failure, while one this
# suite never requested is expected to be absent (on Fedora, zellij has no route
# at all).
if [ "$E2E_WM" = "zellij" ]; then
    E2E_REQUESTED_WMS="tmux zellij"
else
    E2E_REQUESTED_WMS="tmux"
fi

# wm_config_file prints the configuration file the exercised window manager
# writes its default shell into.
wm_config_file() {
    if [ "$E2E_WM" = "tmux" ]; then
        printf '%s\n' "$HOME/.tmux.conf"
    else
        printf '%s\n' "$HOME/.config/zellij/config.kdl"
    fi
}

# check_wm_default_shell <shell> asserts that the exercised window manager has a
# configuration file and that the file names <shell> as its default shell. tmux
# writes `default-shell` into .tmux.conf and zellij writes `default_shell` into
# config.kdl, so the pattern covers either separator.
check_wm_default_shell() {
    expected_shell="$1"
    wm_config=$(wm_config_file)

    if [ ! -f "$wm_config" ]; then
        log_fail "$E2E_WM config not found at $wm_config"
        return
    fi

    log_pass "$E2E_WM config exists"

    if grep -q "default.shell.*$expected_shell" "$wm_config"; then
        log_pass "$E2E_WM config has $expected_shell as its default shell"
    else
        log_fail "$E2E_WM config is missing $expected_shell as its default shell"
    fi
}

# ============================================
# PLATFORM - which terminals this platform supports
# ============================================

# E2E_TERMINALS is every --terminal value in the option space the planning
# matrix walks.
E2E_TERMINALS="alacritty wezterm kitty ghostty none"

# platform_terminals prints the --terminal values this platform supports.
platform_terminals() {
    for terminal in $E2E_TERMINALS; do
        if terminal_supported_on_platform "$terminal"; then
            printf '%s\n' "$terminal"
        fi
    done
}

# terminal_supported_on_platform reports whether --terminal=<value> is a value
# the installer honours here. It mirrors the rule the Go code applies
# (internal/tui/installer.go, supportedTerminals): kitty has a Homebrew cask on
# macOS and no installation route at all on the platforms this suite runs on, so
# asking for it anywhere else is refused rather than planned.
#
# The mirror is checked, not trusted: test_terminal_axis probes the binary for
# every value and fails when the binary disagrees with this rule in either
# direction, so the matrix cannot quietly expect a combination the installer
# would refuse, or accept one it would reject.
terminal_supported_on_platform() {
    case "$1" in
        alacritty|wezterm|ghostty|none)
            return 0
            ;;
        kitty)
            [ "$(uname -s)" = "Darwin" ]
            ;;
        *)
            return 1
            ;;
    esac
}

# ============================================
# BASIC TESTS - Binary functionality
# ============================================

test_binary_runs() {
    log_test "Binary executes with --help"
    if dotfiles --help > /dev/null 2>&1; then
        log_pass "Binary executes correctly"
    else
        log_fail "Binary failed to execute"
    fi
}

test_version() {
    log_test "Binary shows version"
    if dotfiles --version 2>&1 | grep -q "dotfiles"; then
        log_pass "Version displays correctly"
    else
        log_fail "Version not displayed"
    fi
}

test_non_interactive_flag() {
    log_test "Non-interactive flag exists"
    if dotfiles --help 2>&1 | grep -q "non-interactive"; then
        log_pass "Non-interactive mode available"
    else
        log_fail "Non-interactive mode not found"
    fi
}

# ============================================
# INSTALLATION TESTS - Real E2E
# ============================================

# Test: Zsh + the window manager this platform exercises (no nvim, no terminal)
test_zsh_wm() {
    log_test "Install: Zsh + $E2E_WM (no nvim)"
    
    # Run installation (no --test in Docker, container is disposable)
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=zsh --wm="$E2E_WM" --backup=false 2>&1; then
        
        # Verify .zshrc exists
        if [ -f "$HOME/.zshrc" ]; then
            log_pass ".zshrc was created"
        else
            log_fail ".zshrc not found"
            return
        fi
        
        # Verify .zshrc launches the window manager this run selected. The
        # command is a zsh array, so the pattern has to match the array form:
        # the old WM_CMD="tmux" scalar no longer appears in any form. Grepping
        # for the ZELLIJ variable, which is what this test used to do, was
        # vacuous: the shipped .zshrc guards on `-z "$ZELLIJ"` whatever the
        # choice, so that grep passed even when the window manager had never
        # been installed or configured.
        if grep -q "WM_CMD=($E2E_WM" "$HOME/.zshrc"; then
            log_pass ".zshrc launches $E2E_WM"
        else
            log_fail ".zshrc does not launch $E2E_WM"
        fi
        
        # And it must not launch the window manager this run did not select.
        if grep -q "WM_CMD=($E2E_WM_OTHER" "$HOME/.zshrc"; then
            log_fail ".zshrc still launches $E2E_WM_OTHER (expected $E2E_WM)"
        else
            log_pass ".zshrc correctly does not launch $E2E_WM_OTHER"
        fi
        
        # Verify the window manager config has zsh as its default shell
        check_wm_default_shell zsh
    else
        log_fail "Installation failed"
    fi
}

# Test: Fish + Tmux + Nvim
test_fish_tmux_nvim() {
    log_test "Install: Fish + Tmux + Nvim"
    
    # Clean previous test
    rm -rf "$HOME/.config" "$HOME/.zshrc" "$HOME/.tmux.conf" 2>/dev/null || true
    mkdir -p "$HOME/.config"
    
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=fish --wm=tmux --nvim --backup=false 2>&1; then
        
        # Verify fish config exists
        if [ -f "$HOME/.config/fish/config.fish" ]; then
            log_pass "Fish config was created"
        else
            log_fail "Fish config not found"
            return
        fi
        
        # Verify nvim config exists
        if [ -d "$HOME/.config/nvim" ]; then
            log_pass "Neovim config directory created"
        else
            log_fail "Neovim config directory not found"
        fi
        
        # Verify init.lua exists
        if [ -f "$HOME/.config/nvim/init.lua" ]; then
            log_pass "Neovim init.lua exists"
        else
            log_fail "Neovim init.lua not found"
        fi
        
        # Verify tmux config in fish (not zellij)
        if grep -qi "tmux" "$HOME/.config/fish/config.fish"; then
            log_pass "Fish config contains tmux"
        else
            log_fail "Fish config missing tmux"
        fi
        
        # Verify tmux.conf has default-shell set to fish
        if [ -f "$HOME/.tmux.conf" ]; then
            if grep -q 'default-shell.*fish' "$HOME/.tmux.conf"; then
                log_pass "Tmux config has default-shell set to fish"
            else
                log_fail "Tmux config missing default-shell fish"
            fi
        else
            log_fail "Tmux config not found"
        fi
    else
        log_fail "Installation failed"
    fi
}

# Test: Nushell + None WM
test_nushell_no_wm() {
    log_test "Install: Nushell + No WM"
    
    # Clean previous test
    rm -rf "$HOME/.config" "$HOME/.zshrc" 2>/dev/null || true
    mkdir -p "$HOME/.config"
    
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=nushell --wm=none --backup=false 2>&1; then
        
        # Verify nushell config exists
        if [ -d "$HOME/.config/nushell" ]; then
            log_pass "Nushell config directory created"
        else
            log_fail "Nushell config directory not found"
            return
        fi
        
        # Check for config.nu
        if [ -f "$HOME/.config/nushell/config.nu" ]; then
            log_pass "Nushell config.nu exists"
        else
            log_fail "Nushell config.nu not found"
        fi
    else
        log_fail "Installation failed"
    fi
}

# Test: Zsh + Tmux (verify default-shell)
test_zsh_tmux() {
    log_test "Install: Zsh + Tmux (verify default-shell)"
    
    # Clean previous test
    rm -rf "$HOME/.config" "$HOME/.zshrc" "$HOME/.tmux.conf" 2>/dev/null || true
    mkdir -p "$HOME/.config"
    
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=zsh --wm=tmux --backup=false 2>&1; then
        
        # Verify .zshrc exists
        if [ -f "$HOME/.zshrc" ]; then
            log_pass ".zshrc was created"
        else
            log_fail ".zshrc not found"
            return
        fi
        
        # Verify tmux.conf has default-shell set to zsh
        if [ -f "$HOME/.tmux.conf" ]; then
            log_pass "Tmux config exists"
            if grep -q 'default-shell.*zsh' "$HOME/.tmux.conf"; then
                log_pass "Tmux config has default-shell set to zsh"
            else
                log_fail "Tmux config missing default-shell zsh"
            fi
        else
            log_fail "Tmux config not found"
        fi
    else
        log_fail "Installation failed"
    fi
}

# Test: Fish + the window manager this platform exercises (verify default_shell)
test_fish_wm() {
    log_test "Install: Fish + $E2E_WM (verify default shell)"
    
    # Clean previous test
    rm -rf "$HOME/.config" "$HOME/.zshrc" "$HOME/.tmux.conf" 2>/dev/null || true
    mkdir -p "$HOME/.config"
    
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=fish --wm="$E2E_WM" --backup=false 2>&1; then
        
        # Verify fish config exists
        if [ -f "$HOME/.config/fish/config.fish" ]; then
            log_pass "Fish config was created"
        else
            log_fail "Fish config not found"
            return
        fi
        
        # Verify the window manager config has fish as its default shell
        check_wm_default_shell fish
    else
        log_fail "Installation failed"
    fi
}

# Test: Nushell + Tmux (verify default-shell)
test_nushell_tmux() {
    log_test "Install: Nushell + Tmux (verify default-shell)"
    
    # Clean previous test
    rm -rf "$HOME/.config" "$HOME/.zshrc" "$HOME/.tmux.conf" 2>/dev/null || true
    mkdir -p "$HOME/.config"
    
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=nushell --wm=tmux --backup=false 2>&1; then
        
        # Verify nushell config exists
        if [ -d "$HOME/.config/nushell" ]; then
            log_pass "Nushell config directory created"
        else
            log_fail "Nushell config directory not found"
            return
        fi
        
        # Verify tmux.conf has default-shell set to nu
        if [ -f "$HOME/.tmux.conf" ]; then
            log_pass "Tmux config exists"
            if grep -q 'default-shell.*nu' "$HOME/.tmux.conf"; then
                log_pass "Tmux config has default-shell set to nu"
            else
                log_fail "Tmux config missing default-shell nu"
            fi
        else
            log_fail "Tmux config not found"
        fi
    else
        log_fail "Installation failed"
    fi
}

# Test: Nushell + the window manager this platform exercises (verify default_shell)
test_nushell_wm() {
    log_test "Install: Nushell + $E2E_WM (verify default shell)"
    
    # Clean previous test
    rm -rf "$HOME/.config" "$HOME/.zshrc" "$HOME/.tmux.conf" 2>/dev/null || true
    mkdir -p "$HOME/.config"
    
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=nushell --wm="$E2E_WM" --backup=false 2>&1; then
        
        # Verify nushell config exists
        if [ -d "$HOME/.config/nushell" ]; then
            log_pass "Nushell config directory created"
        else
            log_fail "Nushell config directory not found"
            return
        fi
        
        # Verify the window manager config has nu as its default shell
        check_wm_default_shell nu
    else
        log_fail "Installation failed"
    fi
}

# Test: Verify shell is installed and functional
test_shell_functional() {
    log_test "Installed shell is functional"
    
    # Ensure Homebrew is in PATH for this test
    if [ -d "/home/linuxbrew/.linuxbrew/bin" ]; then
        export PATH="/home/linuxbrew/.linuxbrew/bin:$PATH"
    fi
    
    # Check if fish runs
    if command -v fish >/dev/null 2>&1; then
        if fish -c "echo 'fish works'" 2>/dev/null | grep -q "fish works"; then
            log_pass "Fish shell is functional"
        else
            log_fail "Fish shell not working"
        fi
    fi
    
    # Check if zsh runs  
    if command -v zsh >/dev/null 2>&1; then
        if zsh -c "echo 'zsh works'" 2>/dev/null | grep -q "zsh works"; then
            log_pass "Zsh shell is functional"
        else
            log_fail "Zsh shell not working"
        fi
    fi
}

# Test: WM is installed
test_wm_installed() {
    log_test "Window manager is installed"
    
    # Ensure Homebrew is in PATH for this test
    if [ -d "/home/linuxbrew/.linuxbrew/bin" ]; then
        export PATH="/home/linuxbrew/.linuxbrew/bin:$PATH"
    fi
    
    # Assert every window manager this suite asked the installer to install. The
    # previous version only called log_pass when it found one and did nothing
    # when it did not, so a window manager that was requested and never
    # installed produced no verdict at all and the run was counted as a pass.
    # That is how the Fedora job stayed green while zellij had no route to
    # install there.
    for wm in $E2E_REQUESTED_WMS; do
        if command -v "$wm" >/dev/null 2>&1; then
            log_pass "$wm is installed"
        else
            log_fail "$wm was requested by this suite but is not installed"
        fi
    done
}

# Test: Nvim is installed and configured
test_nvim_configured() {
    log_test "Neovim is properly configured"
    
    # Ensure Homebrew is in PATH for this test
    if [ -d "/home/linuxbrew/.linuxbrew/bin" ]; then
        export PATH="/home/linuxbrew/.linuxbrew/bin:$PATH"
    fi
    
    if command -v nvim >/dev/null 2>&1; then
        log_pass "Neovim binary is installed"
        
        # Check if it can start (headless)
        if nvim --headless -c "q" 2>/dev/null; then
            log_pass "Neovim starts successfully"
        else
            log_fail "Neovim failed to start"
        fi
    else
        log_fail "Neovim not installed"
    fi

    # Neovim is configured with clipboard=unnamedplus, so it needs a provider from
    # the system. Without one, yanking inside Neovim silently never reaches the
    # clipboard, which is why a missing provider is asserted rather than assumed.
    if command -v xclip >/dev/null 2>&1; then
        log_pass "Clipboard provider xclip is installed"
    else
        log_fail "Clipboard provider xclip is missing, so Neovim cannot reach the clipboard"
    fi

    if command -v wl-copy >/dev/null 2>&1; then
        log_pass "Clipboard provider wl-clipboard is installed"
    else
        log_fail "Clipboard provider wl-clipboard is missing, so Neovim cannot reach the clipboard"
    fi
}

# ============================================
# OPTION COVERAGE - terminal axis, font, Herdr, planning matrix
# ============================================

# Test: the terminal axis. Every --terminal value is either planned with the step
# it names, or refused on the platform that does not support it, with the values
# that platform does support named in the refusal and no step list planned.
#
# This is the focused case for the terminal axis; the matrix below walks the
# whole option space. It is also what checks the platform rule the matrix uses:
# if the binary accepted kitty off macOS, or refused a value this script
# believes is supported, the disagreement is reported here by name.
test_terminal_axis() {
    log_test "Terminal axis: every --terminal value is planned or refused"

    for terminal in $E2E_TERMINALS; do
        flags="--shell=fish --terminal=$terminal --backup=false"
        if out=$(dotfiles --non-interactive --dry-run $flags 2>&1); then
            rc=0
        else
            rc=$?
        fi

        if terminal_supported_on_platform "$terminal"; then
            if [ "$rc" -ne 0 ]; then
                log_fail "terminal=$terminal: expected a plan, got exit $rc: $out"
                continue
            fi
            if [ "$terminal" = "none" ]; then
                if printf '%s\n' "$out" | grep -qE 'Install .* terminal'; then
                    log_fail "terminal=none planned a terminal step: $out"
                else
                    log_pass "terminal=none plans no terminal step"
                fi
            elif printf '%s\n' "$out" | grep -qF "Install $terminal terminal"; then
                log_pass "terminal=$terminal plans its installation step"
            else
                log_fail "terminal=$terminal was accepted but its step is missing from the plan: $out"
            fi
            continue
        fi

        # Not supported here: the CLI has to refuse before it plans anything.
        if [ "$rc" -eq 0 ]; then
            log_fail "terminal=$terminal is not supported on $(uname -s) but was accepted"
            continue
        fi
        if printf '%s\n' "$out" | grep -q "installation steps"; then
            log_fail "terminal=$terminal was refused but a step list was still planned: $out"
            continue
        fi
        missing=""
        for supported in $(platform_terminals); do
            if ! printf '%s\n' "$out" | grep -qF "$supported"; then
                missing="$missing $supported"
            fi
        done
        if [ -n "$missing" ]; then
            log_fail "terminal=$terminal: refusal does not name supported values:$missing -- got: $out"
            continue
        fi
        log_pass "terminal=$terminal is refused on $(uname -s), naming the supported values"
    done
}

# Test: --font is honoured, and the Nerd Font step is only planned when the flag
# was asked for. The negative control matters: without it, a plan that always
# contained the step would make the first check pass for the wrong reason.
test_font_option() {
    log_test "--font plans the Nerd Font step"

    if out=$(dotfiles --non-interactive --dry-run --shell=fish --wm=none --font --backup=false 2>&1); then
        if printf '%s\n' "$out" | grep -qF "Install Nerd Font"; then
            log_pass "--font plans the Nerd Font step"
        else
            log_fail "--font was accepted but the Nerd Font step is missing from the plan: $out"
        fi
    else
        log_fail "--font failed to plan: $out"
    fi

    if out=$(dotfiles --non-interactive --dry-run --shell=fish --wm=none --backup=false 2>&1); then
        if printf '%s\n' "$out" | grep -qF "Install Nerd Font"; then
            log_fail "the Nerd Font step is planned without --font: $out"
        else
            log_pass "no Nerd Font step without --font"
        fi
    else
        log_fail "planning without --font failed: $out"
    fi
}

# Test: --wm=herdr is honoured. Herdr had no coverage at all: every invocation
# in this suite asked for tmux, zellij or none. The negative control keeps the
# check from passing on a plan that always contains the step.
test_herdr_wm() {
    log_test "--wm=herdr plans the Herdr step"

    if out=$(dotfiles --non-interactive --dry-run --shell=fish --terminal=none --wm=herdr --backup=false 2>&1); then
        if printf '%s\n' "$out" | grep -qF "Install herdr"; then
            log_pass "--wm=herdr plans the Herdr step"
        else
            log_fail "--wm=herdr was accepted but its step is missing from the plan: $out"
        fi
    else
        log_fail "--wm=herdr failed to plan: $out"
    fi

    if out=$(dotfiles --non-interactive --dry-run --shell=fish --terminal=none --wm=none --backup=false 2>&1); then
        if printf '%s\n' "$out" | grep -qF "Install herdr"; then
            log_fail "the Herdr step is planned for --wm=none: $out"
        else
            log_pass "no Herdr step for --wm=none"
        fi
    else
        log_fail "planning for --wm=none failed: $out"
    fi
}

# Test: the dry-run planning matrix over the whole option space.
#
# 3 shells x 5 terminals x 4 window managers x 2 nvim x 2 font = 240
# combinations. Every step returns early under --dry-run (dryRun() guards
# executeStep), so the matrix costs seconds and installs nothing. Per
# combination it asserts an exit code, a non-empty step list and no error text.
#
# Combinations whose terminal this platform does not support are expected to be
# refused by the validation the CLI runs before it plans anything: the matrix
# reflects the decision WU-A made (#16) instead of assuming every combination is
# legal, and every failure names its combination.
test_dry_run_matrix() {
    log_test "Dry-run planning matrix over the option space (240 combinations)"

    matrix_total=0
    matrix_failures=0

    for shell in fish zsh nushell; do
        for terminal in $E2E_TERMINALS; do
            for wm in tmux zellij herdr none; do
                for nvim in 0 1; do
                    for font in 0 1; do
                        matrix_total=$((matrix_total + 1))

                        flags="--shell=$shell --terminal=$terminal --wm=$wm --backup=false"
                        if [ "$nvim" = "1" ]; then
                            flags="$flags --nvim"
                        fi
                        if [ "$font" = "1" ]; then
                            flags="$flags --font"
                        fi

                        if out=$(dotfiles --non-interactive --dry-run $flags 2>&1); then
                            rc=0
                        else
                            rc=$?
                        fi

                        if terminal_supported_on_platform "$terminal"; then
                            if [ "$rc" -ne 0 ]; then
                                matrix_failures=$((matrix_failures + 1))
                                log_fail "matrix: $flags expected a plan, exit $rc: $out"
                            elif ! printf '%s\n' "$out" | grep -qE 'Running [1-9][0-9]* installation steps'; then
                                matrix_failures=$((matrix_failures + 1))
                                log_fail "matrix: $flags planned no steps: $out"
                            elif printf '%s\n' "$out" | grep -qiE 'error|failed'; then
                                matrix_failures=$((matrix_failures + 1))
                                log_fail "matrix: $flags printed an error while planning: $out"
                            fi
                        else
                            if [ "$rc" -eq 0 ]; then
                                matrix_failures=$((matrix_failures + 1))
                                log_fail "matrix: $flags must be refused on $(uname -s), but it was accepted"
                            elif printf '%s\n' "$out" | grep -q "installation steps"; then
                                matrix_failures=$((matrix_failures + 1))
                                log_fail "matrix: $flags was refused but still planned steps: $out"
                            fi
                        fi
                    done
                done
            done
        done
    done

    if [ "$matrix_failures" -eq 0 ]; then
        log_pass "matrix: all $matrix_total combinations planned or were refused as expected"
    else
        log_fail "matrix: $matrix_failures of $matrix_total combinations failed; each failing combination is named above"
    fi
}

# ============================================
# BACKUP SYSTEM TESTS
# ============================================

# Helper: Create fake existing configs
setup_fake_configs() {
    log_test "Setting up fake existing configs..."
    
    # Create fake nvim config
    mkdir -p "$HOME/.config/nvim"
    echo "-- Fake nvim config" > "$HOME/.config/nvim/init.lua"
    echo "vim.opt.number = true" >> "$HOME/.config/nvim/init.lua"
    
    # Create fake fish config
    mkdir -p "$HOME/.config/fish"
    echo "# Fake fish config" > "$HOME/.config/fish/config.fish"
    echo "set -x EDITOR nvim" >> "$HOME/.config/fish/config.fish"
    
    # Create fake .zshrc
    echo "# Fake zshrc" > "$HOME/.zshrc"
    echo "export EDITOR=nvim" >> "$HOME/.zshrc"
    
    # Create fake tmux config
    echo "# Fake tmux config" > "$HOME/.tmux.conf"
    echo "set -g prefix C-a" >> "$HOME/.tmux.conf"
    
    # Create fake zellij config  
    mkdir -p "$HOME/.config/zellij"
    echo "// Fake zellij config" > "$HOME/.config/zellij/config.kdl"
    
    log_pass "Fake configs created"
}

# Helper: Cleanup test environment
cleanup_test_env() {
    rm -rf "$HOME/.config/nvim" 2>/dev/null || true
    rm -rf "$HOME/.config/fish" 2>/dev/null || true
    rm -rf "$HOME/.config/zellij" 2>/dev/null || true
    rm -f "$HOME/.zshrc" 2>/dev/null || true
    rm -f "$HOME/.tmux.conf" 2>/dev/null || true
    rm -rf "$HOME/.dotfiles-backup-"* 2>/dev/null || true
}

# Test: Existing configs are detected
test_detect_existing_configs() {
    log_test "Detecting existing configurations"
    
    cleanup_test_env
    setup_fake_configs
    
    # The installer should detect these configs
    # We verify by checking if the files exist
    detected=0
    
    if [ -f "$HOME/.config/nvim/init.lua" ]; then
        detected=$((detected + 1))
    fi
    if [ -f "$HOME/.config/fish/config.fish" ]; then
        detected=$((detected + 1))
    fi
    if [ -f "$HOME/.zshrc" ]; then
        detected=$((detected + 1))
    fi
    if [ -f "$HOME/.tmux.conf" ]; then
        detected=$((detected + 1))
    fi
    if [ -f "$HOME/.config/zellij/config.kdl" ]; then
        detected=$((detected + 1))
    fi
    
    if [ $detected -eq 5 ]; then
        log_pass "All 5 fake configs detected ($detected/5)"
    else
        log_fail "Expected 5 configs, found $detected"
    fi
}

# Test: Backup creation works
test_backup_creation() {
    log_test "Creating backup of existing configs"
    
    cleanup_test_env
    setup_fake_configs
    
    # Run installer with backup=true
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=fish --wm=tmux --backup=true 2>&1; then
        
        # Check if backup directory was created
        backup_count=$(ls -d "$HOME/.dotfiles-backup-"* 2>/dev/null | wc -l)
        
        if [ "$backup_count" -gt 0 ]; then
            log_pass "Backup directory created"
            
            # Get the backup directory
            backup_dir=$(ls -d "$HOME/.dotfiles-backup-"* 2>/dev/null | head -1)
            
            # Check if files were backed up
            if [ -d "$backup_dir" ]; then
                files_backed=$(ls "$backup_dir" 2>/dev/null | wc -l)
                if [ "$files_backed" -gt 0 ]; then
                    log_pass "Backup contains $files_backed items"
                else
                    log_fail "Backup directory is empty"
                fi
            fi
        else
            log_fail "No backup directory found"
        fi
    else
        log_fail "Installation with backup failed"
    fi
}

# Test: Backup directory naming format
test_backup_naming() {
    log_test "Backup directory naming format"
    
    # Check existing backups match pattern: .dotfiles-backup-YYYY-MM-DD-HHMMSS
    backup_dirs=$(ls -d "$HOME/.dotfiles-backup-"* 2>/dev/null || echo "")
    
    if [ -n "$backup_dirs" ]; then
        for dir in $backup_dirs; do
            basename=$(basename "$dir")
            # Check if format matches .dotfiles-backup-YYYY-MM-DD-HHMMSS
            if echo "$basename" | grep -qE '^\.dotfiles-backup-[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9]{6}$'; then
                log_pass "Backup naming format correct: $basename"
            else
                log_fail "Backup naming format incorrect: $basename"
            fi
        done
    else
        log_pass "No backups to check (expected in some test runs)"
    fi
}

# Test: Restore from backup works
test_backup_restore() {
    log_test "Restoring from backup"
    
    cleanup_test_env
    setup_fake_configs
    
    # First, create a backup
    mkdir -p "$HOME/.dotfiles-backup-test-restore"
    
    # Copy files to backup manually for this test
    mkdir -p "$HOME/.dotfiles-backup-test-restore/nvim"
    echo "-- Original nvim config from backup" > "$HOME/.dotfiles-backup-test-restore/nvim/init.lua"
    echo "-- This should be restored" >> "$HOME/.dotfiles-backup-test-restore/nvim/init.lua"

    # Now modify the current config (simulate overwrite by installer)
    echo "-- New config after install" > "$HOME/.config/nvim/init.lua"
    
    # Verify modification happened
    if grep -q "New config after install" "$HOME/.config/nvim/init.lua"; then
        log_pass "Config was modified (simulating install)"
    else
        log_fail "Could not modify config for test"
        return
    fi
    
    # Now restore from backup manually (simulating restore function)
    if cp "$HOME/.dotfiles-backup-test-restore/nvim/init.lua" "$HOME/.config/nvim/init.lua"; then
        # Verify restore worked
        if grep -q "Original nvim config from backup" "$HOME/.config/nvim/init.lua"; then
            log_pass "Backup restore successful"
        else
            log_fail "Restored content doesn't match original"
        fi
    else
        log_fail "Failed to copy from backup"
    fi
    
    # Cleanup test backup
    rm -rf "$HOME/.dotfiles-backup-test-restore"
}

# Test: Multiple backups can coexist
test_multiple_backups() {
    log_test "Multiple backups can coexist"
    
    # Create multiple fake backups with different timestamps
    mkdir -p "$HOME/.dotfiles-backup-2024-01-15-120000"
    echo "backup1" > "$HOME/.dotfiles-backup-2024-01-15-120000/test"
    
    mkdir -p "$HOME/.dotfiles-backup-2024-01-16-130000"
    echo "backup2" > "$HOME/.dotfiles-backup-2024-01-16-130000/test"
    
    mkdir -p "$HOME/.dotfiles-backup-2024-01-17-140000"
    echo "backup3" > "$HOME/.dotfiles-backup-2024-01-17-140000/test"
    
    # Count backups
    backup_count=$(ls -d "$HOME/.dotfiles-backup-"* 2>/dev/null | wc -l)
    
    if [ "$backup_count" -ge 3 ]; then
        log_pass "Multiple backups coexist ($backup_count found)"
    else
        log_fail "Expected at least 3 backups, found $backup_count"
    fi
    
    # Cleanup test backups
    rm -rf "$HOME/.dotfiles-backup-2024-01-15-120000"
    rm -rf "$HOME/.dotfiles-backup-2024-01-16-130000"
    rm -rf "$HOME/.dotfiles-backup-2024-01-17-140000"
}

# Test: Backup deletion works
test_backup_deletion() {
    log_test "Backup deletion"
    
    # Create a test backup
    test_backup="$HOME/.dotfiles-backup-delete-test"
    mkdir -p "$test_backup"
    echo "test" > "$test_backup/testfile"
    
    # Verify it exists
    if [ ! -d "$test_backup" ]; then
        log_fail "Could not create test backup"
        return
    fi
    
    # Delete it
    rm -rf "$test_backup"
    
    # Verify deletion
    if [ ! -d "$test_backup" ]; then
        log_pass "Backup deletion successful"
    else
        log_fail "Backup still exists after deletion"
    fi
}

# Test: Install without backup doesn't create backup dir
test_install_no_backup() {
    log_test "Install without backup doesn't create backup"
    
    cleanup_test_env
    setup_fake_configs
    
    # Count existing backups before
    before_count=$(ls -d "$HOME/.dotfiles-backup-"* 2>/dev/null | wc -l)
    
    # Run installer with backup=false
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=zsh --wm=none --backup=false 2>&1; then
        
        # Count backups after
        after_count=$(ls -d "$HOME/.dotfiles-backup-"* 2>/dev/null | wc -l)
        
        if [ "$after_count" -eq "$before_count" ]; then
            log_pass "No new backup created when backup=false"
        else
            log_fail "Backup was created despite backup=false"
        fi
    else
        log_fail "Installation failed"
    fi
}

# Test: Backup contains expected config files
test_backup_contents() {
    log_test "Backup contains expected files"
    
    cleanup_test_env
    setup_fake_configs
    
    # Run installer with backup
    if DOTFILES_VERBOSE=1 dotfiles --non-interactive \
        --shell=fish --wm=tmux --nvim --backup=true 2>&1; then
        
        # Find the backup
        backup_dir=$(ls -dt "$HOME/.dotfiles-backup-"* 2>/dev/null | head -1)
        
        if [ -d "$backup_dir" ]; then
            # List what's in the backup
            log_test "Backup contents:"
            ls -la "$backup_dir" 2>/dev/null || true
            
            # Check for expected items (at least some should be there)
            found_items=0
            
            # Note: The backup system uses config keys, not full paths
            for item in nvim fish zsh tmux zellij; do
                if [ -e "$backup_dir/$item" ]; then
                    found_items=$((found_items + 1))
                    log_pass "  Found: $item"
                fi
            done
            
            if [ $found_items -gt 0 ]; then
                log_pass "Backup contains $found_items config items"
            else
                log_fail "Backup is empty or missing expected items"
            fi
        else
            log_fail "Could not find backup directory"
        fi
    else
        log_fail "Installation with backup failed"
    fi
}

# ============================================
# RUN TESTS
# ============================================

log_section "dotfiles E2E Tests"

# Basic tests first
log_section "Basic Tests"
test_binary_runs
test_version
test_non_interactive_flag

# Option coverage. Planning only, so it runs on every platform, including the
# basic (non-RUN_FULL_E2E) images, and installs nothing.
log_section "Option Coverage (dry run)"
test_terminal_axis
test_font_option
test_herdr_wm
test_dry_run_matrix

# Backup tests (can run in basic mode)
if [ "$RUN_BACKUP_TESTS" = "1" ] || [ "$RUN_FULL_E2E" = "1" ]; then
    log_section "Backup System Tests"
    test_detect_existing_configs
    test_backup_creation
    test_backup_naming
    test_backup_restore
    test_multiple_backups
    test_backup_deletion
    test_install_no_backup
    test_backup_contents
    
    # Cleanup after backup tests
    cleanup_test_env
fi

# Installation tests (only if we have the full environment)
if [ "$RUN_FULL_E2E" = "1" ]; then
    log_section "Installation Tests"
    test_zsh_wm
    test_fish_tmux_nvim
    test_nushell_no_wm
    
    log_section "Shell + WM Default Shell Tests"
    test_zsh_tmux
    test_fish_wm
    test_nushell_tmux
    test_nushell_wm
    
    log_section "Verification Tests"
    test_shell_functional
    test_wm_installed
    test_nvim_configured
fi

# Summary
log_section "Test Summary"
printf "  ${GREEN}Passed: %d${NC}\n" "$PASSED"
printf "  ${RED}Failed: %d${NC}\n" "$FAILED"

if [ $FAILED -gt 0 ]; then
    printf "\n${RED}SOME TESTS FAILED${NC}\n"
    exit 1
else
    printf "\n${GREEN}ALL TESTS PASSED${NC}\n"
    exit 0
fi
