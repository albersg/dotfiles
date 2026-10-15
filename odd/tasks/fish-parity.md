# The fish shell's parity with zsh

## Goal

Make fish the shell zsh already is: the same file listing, the same day-to-day
functions, and the same secrets and project hooks — each one installed by the
installer rather than typed by hand.

The gap is not taste, and it is not a plugin zsh has that fish lacks. It is that
`dotfiles-zsh/.zshrc` was built out and `dotfiles-fish/fish/config.fish` was not.

## Evidence, measured on this machine (read-only)

Machine: WSL2 Debian, Windows Terminal, herdr, dotfiles v0.7.10. `eza 0.23.5` is
installed at `/home/linuxbrew/.linuxbrew/bin/eza` and **is already on the fish
`PATH`** (`fish -c 'command -v eza'` prints that path), so fish has the tool and
never calls it.

| what | zsh | fish |
|------|-----|------|
| `ls` | `eza --icons --group-directories-first` | `gls --color=auto` |
| `ll`, `la`, `lt`, `tree` | all four defined | **none** |
| `EZA_ICON_SPACING` | `2` | **not set** |
| `LS_COLORS`, `EZA_COLORS` | from the generated theme block | **not set** |
| `cat` → `bat` for plain invocations | yes | **no** |
| `extract`, `compress`, `serve` | yes | **no** |
| sops/age secrets helpers | `env-edit`, `env-load`, `env-export` | **no** |
| venv activation on `cd` | `add-zsh-hook chpwd _dotfiles_activate_venv` | **no** |
| project-task announcement on `cd` | `add-zsh-hook chpwd _dotfiles_show_tasks` | **no** |

`fish -ic 'type ls'` on this machine prints the function the alias builds:
`gls --color=auto $argv`. That is the whole of fish's listing configuration.

### D1 — fish's `ls` prints no icons because it never calls `eza`

The user-visible half. `eza` is named by `macos.yaml`, `linux.yaml`, `Brewfile:45`
and the Arch and Fedora columns of `shellPlatformPackages`, and Debian gets it from
Homebrew, which this installer puts in place for Debian hosts
(`installer.go`, the `case "zsh"` comment). So `eza` is a tool every supported host
is expected to have, and fish simply does not use it.

### D2 — the fallback alias would break on a host without `gls`

`alias ls='gls --color=auto'` is the Linux branch and names `gls` unconditionally.
`gls` is GNU coreutils' prefixed binary and only Homebrew installs it under that
name; a Debian host whose only `ls` is the system one has no `gls`, and the alias
fails on every call. This is the same defect the zsh configuration already guards
against for `trip` and `btop` — an alias for a tool that may be absent is only
declared when the tool resolves.

### D3 — fish's listing has none of the palette zsh's listing has

`LS_COLORS` (which `eza` reads as its base layer), `EZA_COLORS` (metadata) and
`EZA_ICON_SPACING` are set for zsh inside and just below the generated theme block.
`renderFishConfig` emits `fish_color_*` and `fish_pager_color_*` only, so fish's
listing is `eza`'s own default sixteen colours, not the theme's. Stage 2, below.

### Where it happens

- `dotfiles-fish/fish/config.fish`, the `## alias` block — D1 and D2.
- `renderFishConfig` (`installer/internal/tui/installer.go:2894`) — D3. The zsh
  half is `renderZshTheme` (`:3446`) with `zshLsColorsBlock` (`:3513`) and
  `zshEzaColorsBlock` (`:3541`); those two tables are written as literal strings
  in the zsh renderer's own vocabulary, and fish cannot read them as they stand.

## What is already right, and must stay right

- **`eza` is installed by the toolset step, not by this front.** Nothing here adds
  a package: the front is that fish does not call a tool it already has.
- **The `dotfiles-theme: fish` block is generated and must stay generated.** The
  listing aliases belong outside it, next to the other hand-written aliases, or
  the theme switch's `AdoptStart`/`AdoptEnd` markers stop delimiting it.
- **A fish alias is not a zsh alias.** They are two different configurations that
  must agree on the *policy* — which tool lists files, with which flags — not on
  the text. The guard compares the flags, and nothing else.
- **`command -q eza` is the guard, not a version test.** The repository's rule for
  an alias whose tool may be missing is to ask whether it resolves.

## Tasks

Stage 1 — the listing (this change):
1. Give fish the five listing aliases, guarded on `eza`, with the flags zsh uses.
2. Keep a colour `ls` for a host where `eza` does not resolve, and guard `gls`.
3. Set `EZA_ICON_SPACING` for fish, so the icons have the space zsh gives them.
4. Guard the flags: a flag changed in one shell and not the other fails the guard.

Stage 2 — the listing's colours (its own change):
5. Emit `LS_COLORS`, `EZA_COLORS` and the icon spacing from one mapping both
   renderers read, instead of a zsh-shaped literal fish cannot use.

Stage 3 — the functions: `cat` → `bat`, `extract`, `compress`, `serve`.

Stage 4 — the hooks: sops/age secrets, the venv activation on `cd`, and the
project-task announcement.

## The proof

Stage 1. Every line below was run; nothing here is intended to be run.

| what | value |
|------|-------|
| the three guards against the unmodified `config.fish` | **all three fail** |
| `TestFishListingAliasesMatchTheOnesZshDefines` | fails with `read no \`eza\` alias from ../../../dotfiles-fish/fish/config.fish: fish lists files without eza, so its \`ls\` prints no icons while zsh's does` |
| `TestFishGuardsItsListingAliasesOnEza` | fails with `declares eza aliases without asking whether eza resolves, so a host that has not got it gets an alias that fails on every call` |
| `TestFishKeepsAColourLsWithoutEza` | fails with ``aliases `ls` to `gls --color=auto` without asking whether gls resolves, so a host without Homebrew's coreutils gets an alias that fails on every call`` |
| the three guards after the change | pass |
| mutation: fish's `tree` `--level=3` changed to `--level=2` | fails, naming `tree` and both flag sets verbatim |
| mutation: `--git` dropped from fish's `ll` | fails, naming `ll` and both flag sets verbatim |
| mutation: the Linux `gls` fallback deleted | `TestFishKeepsAColourLsWithoutEza` fails, naming `gls --color=auto` |
| mutation: `if command -q eza` weakened to `if true` | `TestFishGuardsItsListingAliasesOnEza` fails |
| the configuration restored after every mutation | byte-identical to the version under test, checked with `diff -q` |
| `fish -n` on the rewritten `config.fish` | syntax OK |
| the listing block sourced in a clean `fish --no-config` | all five aliases resolve to the eza command zsh's are, and `EZA_ICON_SPACING` is `2` |
| `gofmt -l ./internal ./cmd` | no output |
| `go vet ./...` | exit 0 |
| `go build ./...` | ok |
| `go test ./internal/tui/ -count=1`, this branch | 4 failures |
| the same command on a detached worktree of `origin/main` (`bdbdaa4`) | the same 4 failures |
| the two failure sets compared | **identical**: this change adds none |

### A guard whose input is a file outside its package needs `-count=1`

This cost one wrong measurement and is worth writing down. The first mutation run reported
`0` for every mutation **including the baseline**, which reads as "the guard has no teeth".
It had teeth; the run was reading Go's test cache. Go caches **successful** test results
keyed by the package's sources and the test flags, and `dotfiles-fish/fish/config.fish` is
neither: the guard reads it at run time from outside `installer/internal/tui`. So every
mutation replayed the cached pass.

The tell is the other direction: the *first* run, against the unmodified file, was genuinely
fresh and genuinely failed, because Go does not cache failures. Any run of this guard that
has to be trusted — a mutation study, a pre-merge check — passes `-count=1`.

### The package suite already fails, and the failures are not this change's

`go test ./internal/tui/` reports **four failures on this branch and the same four on a clean
`origin/main`**, compared by name:

```
TestStepInstallTerminalGhosttyReportsAFailedDownload
TestStepInstallTerminalGhosttyRunsTheDownloadItFetches
TestInteractiveGhosttyScriptFailsWhenTheDownloadFails
TestInteractiveGhosttyScriptDownloadsThenRuns
```

They are the Ghostty failures `font-windows-truth.md` and `theme-survives-install.md`
already record. The comparison was run as two suites in parallel, one against this tree and
one against a `git worktree add --detach /tmp/dotfiles-baseline origin/main`, so the only
difference between the runs was this change; the worktree was removed afterwards.

Two traps are worth naming, because both produced a wrong reading first:

- **`go test` output is not a failure list.** The run prints the captured stdout of tests
  that *pass*, and two of those read like disasters — a step summary carrying
  `Configure WSL ... ❌ FAILED ... permission denied`, and another carrying
  `zsh installation failed`. Both are assertions about what a *simulated* failure
  produces, printed by tests that pass. Only lines beginning `--- FAIL` are failures, which
  is why the comparison counts those rather than reading the tail.
- **A one-line `tail` destroys the comparison.** The first run of this suite captured only
  its last 40 lines, so the failure list could not be reconstructed without running the
  suite again. Any run whose output will be compared later is captured whole.

## Stage 2 — the listing's colours

The mandate is stage 1's own task 5, quoted verbatim: *"Emit `LS_COLORS`,
`EZA_COLORS` and the icon spacing from one mapping both renderers read, instead of
a zsh-shaped literal fish cannot use."* The icon spacing is already done — stage 1
wrote `set -gx EZA_ICON_SPACING 2` at `config.fish:115`, outside the generated
region. What is missing is the two tables.

### What was measured before writing anything

- **fish reads what zsh writes; only the spelling has to change.** A probe run
  through `fish --no-config` on this machine, declaring two palette variables and
  the two tables with them:

  ```fish
  set -g PALETTE_BLUE_SGR "38;2;1;2;3"
  set -gx LS_COLORS "rs=0:\
  di=$PALETTE_BLUE_SGR:\
  ex=$PALETTE_RED_SGR"
  ```

  printed `rs=0:di=38;2;1;2;3:ex=38;2;4;5;6` as **one** line (a newline count of
  `0`), and `env` carried the same string. So `$VAR` expands inside fish's double
  quotes, a backslash at the end of a line continues the value without keeping the
  newline, and `set -gx` exports it for `eza` to read out of the environment — the
  same three things zsh's spelling buys.
- **Nothing guards either generated block.** `grep -l renderZshTheme renderFishConfig`
  over `installer/internal/tui/*_test.go` returns nothing: the stage-1 guard reads
  `config.fish` as a file and never renders it. A refactor of the zsh renderer
  therefore has no test to catch a change, which is why S6 is guarded here.
- **The sentinel path is the one that runs.** `replaceThemeBlock`
  (`installer.go:1342`) is tried before `adoptThemeBlock`, and `config.fish`
  carries both markers (`:121`, `:149`). The fish artifact's `AdoptStart`/`AdoptEnd`
  anchors (`set -l foreground F3F6F9 normal` / `set -g fish_pager_color_description $comment`,
  `installer.go:1293`) are the hand-written-to-generated bridge only, so stage 2
  leaves them alone.
- **The two blocks as they stand**, rendered from every theme in `themes/` before
  the change and kept for comparison: zsh 91 lines, 6567-6575 bytes; fish 27 lines,
  1533-1541 bytes. Both tables live in the zsh block, as literal strings
  (`zshLsColorsBlock`, `installer.go:3513`; `zshEzaColorsBlock`, `:3541`) whose every
  value is a `${PALETTE_*_SGR}` reference.

### Specs

S1. For every theme in `themes/`, the fish block's `LS_COLORS` and `EZA_COLORS` carry
    the same mapping the zsh block's do: the same keys, in the same order, each
    taking the same palette role.
S2. The mapping is written once. No key is stated in a shell's own renderer, so a key
    added to the mapping reaches both shells and neither can drift from the other.
S3. The fish tables read the theme's own values, computed the way zsh's are: the SGR
    sequence of the palette role the mapping names, so a switch moves both shells and
    neither falls back to a built-in sixteen.
S4. What fish is given is fish. The emitted block holds no `${VAR}` and no `export `,
    and declares every variable its tables read before reading it.
S5. The fish block stays the generated region: still inside the two sentinels, still
    replaced whole by a switch, and nothing outside it moves — the stage-1 aliases,
    the `EZA_ICON_SPACING` line and the hand-written functions are untouched.
S6. The zsh block is byte-identical before and after. This change moves *where* the
    palette is computed, not what zsh is given.

### Tasks

T1. Compute the palette once, for every renderer that reads it.
T2. Render the two listing tables from the single mapping, in each shell's spelling.
T3. Emit the palette and both tables in the fish block, above the fish roles.
T4. Guard it: the two shells agree on mapping and values, and fish is given fish.

### The proof (stage 2)

| what | value |
|------|-------|
| the three guards against the unmodified renderer | **all three fail**: `the fish block declares no LS_COLORS at all, so LS_COLORS reaches the tool unset and the listing falls back to that tool's own colours: the fish block holds 1541 bytes and no listing table` |
| the same three after the change | pass, over all seven definitions in `themes/` |
| the two shells compared entry for entry | 67 `LS_COLORS` entries and 66 `EZA_COLORS` entries, identical under both shells |
| mutation: the fish table loses one entry | `TestBothShellsListFilesFromTheSameMapping` fails naming both sides: `zsh's LS_COLORS carries 67 entries and fish's carries 66`, and `entry 0 of LS_COLORS is "rs=0" under zsh and "di=38;2;30;102;245" under fish` |
| mutation: fish is handed zsh's `export` untranslated | the mapping guard fails (`declares no LS_COLORS at all ... the fish block holds 6120 bytes and no listing table`) and the shape guard fails with it |
| mutation: both halves paint a colour the theme never chose | **only** `TestFishListingTablesPaintTheThemesOwnValues` fails -- `di is painted "38;2;1;1;1", which is not a value in themes/catppuccin-latte.toml` -- while the mapping guard stays green, because both shells moved together |
| each mutation restored afterwards | byte-identical to the version under test, checked with `diff -q` |
| the zsh block, rendered for three themes before and after | **identical byte for byte**, all three (91 lines, 6567-6575 bytes) |
| `fish -n` on the rendered block | syntax OK |
| the block sourced in `fish --no-config` | `LS_COLORS` resolves to 67 entries, `EZA_COLORS` to 66, `di` is the theme's blue, and `env` carries `LS_COLORS`, so eza reads it |
| `dotfiles-fish/fish/config.fish`, regenerated | +68 lines and no deletions: the palette and the two tables, inside the same two sentinels, with `fish_pager_color_description` still the block's last line |
| `TestGeneratedThemeArtifactsMatchTheirDefinition` before regenerating | **fails**: `dotfiles-fish/fish/config.fish no longer matches themes/dotfiles.toml: regenerate it with go test ./internal/tui -run TestGeneratedThemeArtifactsMatchTheirDefinition -update-theme-artifacts` |
| the same guard after regenerating | passes; the other thirteen artifacts already matched byte for byte |
| `gofmt -l ./internal ./cmd` | no output |
| `go vet ./...` | exit 0 |
| the full suite, unfiltered, before regenerating | **5 failures**: the 4 Ghostty ones and the artifact guard above |
| the full suite, unfiltered, after regenerating | **the 4 Ghostty failures only**, the same four this file and `theme-survives-install.md` already record |

### Log

**L1.** The mandate is this file's task 5, quoted verbatim above. Nothing was added to
it: the icon spacing was already done, outside the generated region, by stage 1.

**L2.** fish's parser, measured rather than reasoned about (`fish --no-config`):

```
expandido -> [rs=0:di=38;2;1;2;3:ex=38;2;4;5;6]
lineas = 0
LS_COLORS=rs=0:di=38;2;1;2;3:ex=38;2;4;5;6
```

A bare `$VAR` expands inside a double-quoted value, the backslash at the end of a line
continues that value without keeping the newline (`lineas = 0`), and `set -gx` exports
the result.

**L3.** The space before the quote is load-bearing, and the naive translation is what
measured it:

```
set: A=uno:dos: invalid variable name. See `help language#shell-variable-and-function-names`
set: Did you mean `set A uno:dos`?
```

So `export NAME="` cannot become `set -gx NAME="`; fish refuses it. The translation
emits `set -gx NAME "` and the value as an argument of its own.

**L4.** **A wrong measurement, recorded because it was acted on before it was caught.**
A search for the renderers by name -- `grep -l renderZshTheme renderFishConfig` over
`installer/internal/tui/*_test.go` -- returns nothing, and I concluded that no test
guarded either generated block, so the shipped artifact could be left alone. The suite
said otherwise: `TestGeneratedThemeArtifactsMatchTheirDefinition`
(`install_paths_test.go:2665`) renders every artifact through the same table the
installer uses and compares it with the committed file, and it failed naming
`dotfiles-fish/fish/config.fish`. The search was for an identifier; the guard works
through the artifact's `Render` field. What was true is narrower: nothing calls the
renderers *by name*.

**L5.** The regression is regenerated, never hand-edited: the guard's own message
carries the command (`-update-theme-artifacts`) and it goes through
`themeArtifactBlock` + `replaceThemeBlock`, the path the installer takes.

**L6.** Design: the block declares the palette's SGR values and renders both tables from
the two constants zsh's block is rendered from, rather than inlining the sequences into
the tables. Inlining would have been fewer lines and no global variables, and it was
rejected because it hides what this stage is -- one mapping, read by both shells -- and
because it stops the two blocks from being read side by side.

**L7.** Order inside the block: the palette, then the tables, then fish's eighteen roles.
The roles stay last because `installer_test.go:72` uses `set -g fish_pager_color_description`
as the end of the shipped block when it checks that the drop-in loader comes after it.

**L8.** The values guard allows exactly one entry to name no palette colour: `rs=0` is
SGR's reset and is in the table on purpose. Any other entry without one fails.

**L9.** A mutation that does not compile measures nothing. The first form of the third
mutation -- forcing every SGR value to a constant -- left a local variable unused, so
the package failed to build and the guards never ran; it was rewritten so that both
halves receive the same value outside the theme, which is what makes only the values
guard fire.

**L10.** Stage 2 was merged as `f0d0cbf` (`feat(fish): the listing is painted by the
theme, not by eza's own defaults`, PR #324), and its guards were re-run on the merged
`main` rather than on the branch, which is the tree a user gets: `themeShellPaletteFor`
5 occurrences, `themeListingTableForShell` 4, three new test functions, all green.

## Stage 3 — the functions

The mandate is this file's own stage 3: *"the functions: `cat` → `bat`, `extract`,
`compress`, `serve`."*

### What was measured before writing anything

Read out of `dotfiles-zsh/.zshrc`; the line numbers are that file's.

| function | zsh | what it does |
|----------|-----|--------------|
| `cat` | `:442-451` | a function rather than an alias, so a script does not inherit it: an argument beginning with `-` goes to `command cat`, which is what keeps the flags working, and a plain invocation goes to `bat --paging=never --style=plain --` |
| `extract` | `:546-581` | one archive: thirteen `case` patterns onto eleven tools, the tool named in the message before it is run |
| `compress` | `:586-658` | an archive and one or more paths: five archive formats and the four single-file ones, which refuse more than one path or a directory, and remove the target when the compressor fails |
| `serve` | `:732-752` | a port (`8000` by default) validated as a whole number in 1-65535, python3 checked, the port checked for use, then `python3 -m http.server` bound to `127.0.0.1` |

- **`start_if_needed` (`:30`) is out of scope.** It is zsh's own, the parity table that
  opened this file never listed it, and this stage's mandate names four functions. It is
  recorded here as a lead rather than ported in silence.
- **fish's primitives were measured, not assumed.** Running them through
  `fish --no-config` confirmed what the translation rests on: `case` takes several glob
  patterns as alternatives and `|` inside one pattern is a literal; `$argv[2..-1]` is the
  rest of the arguments; `command -q` is the guard; `string match -qr` tests the port;
  a command held in a variable runs as a command; `>&2` sends a message to the error
  stream and `return 1` sets the status.

### Specs

S1. fish declares the same four commands zsh declares, under the same names: `cat`,
    `extract`, `compress`, `serve`.
S2. `cat` keeps zsh's policy: an argument beginning with `-` reaches the real `cat`, so
    flags are never silently swallowed, and a plain invocation reaches `bat` with the
    same three flags. It is declared as a function, so a script does not inherit it.
S3. `extract` names the same tool for the same format as zsh's does, for every pattern
    zsh handles, and asks whether that tool resolves before running it, so a host
    without it is told which one is missing instead of getting a bare
    `command not found`.
S4. `compress` writes the same formats with the same tool and the same flags, refuses a
    single-file target given several paths, a directory or a missing file, and removes
    the target when the compressor fails, so a failure never leaves an empty archive
    and a success reported.
S5. `serve` validates the port the way zsh's does -- a whole number, 1 to 65535 -- asks
    whether python3 resolves, refuses a port already in use, and serves the current
    directory on `127.0.0.1`.
S6. The messages a person reads are the same strings in both shells, so which shell is
    running never changes what a mistake says.
S7. Nothing outside these four functions moves: the generated theme block, stage 1's
    listing aliases, `EZA_ICON_SPACING` and the drop-in loader stay as they are.

### Tasks

T1. Declare the four functions in `config.fish`, outside the generated block.
T2. Guard it: the four names, the tool per format, the guards on the tools, and the
    messages are compared against `.zshrc`, so one shell cannot drift from the other.
T3. Prove it with behaviour, not only by reading: the functions are run in a real fish
    against real archives.

### The proof (stage 3)

| what | value |
|------|-------|
| the seven guards against `config.fish` as it stands in `HEAD` | **all seven fail**, each naming the missing command: `declares no cat, so fish is missing a command zsh has: cat, extract, compress, serve`, `declares no extract, so every archive zsh unpacks has to be unpacked by hand under fish`, `declares no compress, so none of the archives zsh writes can be written under fish`, `declares no serve, so it has no message of its own for this guard to compare` |
| the same seven after the change | pass; `fish -n` accepts the file |
| the four functions run in a real fish against real files | **20 checks, all correct**: `cat` prints bat's plain output (`hola`) and `cat -n` numbers it (`1\thola`), `extract` unpacks a real `.tar.gz`, `compress` writes a `.gz` that passes `gzip -t`, and `extract` unpacks that file back |
| the tool guard, exercised rather than read | with `PATH` set to a directory that does not exist: `extract: needs tar, which is not installed` and status 1 -- not a bare command-not-found |
| every refusal in fish carries zsh's own string | `compress one.gz a.txt dir`, `compress dir.gz dir`, `compress out.gz missing.txt`, `compress out.bin a.txt`, `extract weird.bin` (an existing file of unknown format), `serve abc`, `serve 99999`, and each of the two usage lines |
| mutation: fish loses `serve` | `TestFishDeclaresTheCommandsZshDeclares` fails: `declares no serve, so fish is missing a command zsh has: cat, extract, compress, serve` |
| mutation: a flag changes (`tar -xzf` -> `-xf`) | `TestBothShellsExtractTheSameFormatsWithTheSameTools` fails naming both sides: `extract runs "tar -xzf" for *.tar.gz under zsh and "tar -xf" under fish` |
| mutation: a message changes (`no such file` -> `not found`) | `TestTheErrorMessagesAreTheSameInBothShells` fails printing both sets, `[extract: no such file: $...]` against `[extract: not found: $...]` |
| mutation: the tool check is removed | `TestExtractAndCompressAskWhetherTheToolResolves` fails: `runs a tool without asking command -q whether it resolves, so a host without it gets a bare command-not-found instead of being told which tool is missing` |
| mutation: **the wiring** (`.tar.bz2` running `.xz`'s `tar -xJf`) | `TestBothShellsExtractTheSameFormatsWithTheSameTools` fails naming both sides -- the case the comparison exists for, and the reason it compares the tool and the flags per pattern instead of comparing the two tables as text |
| mutation: `bat` loses `--style=plain` | `TestCatKeepsItsPolicyInBothShells` fails: `cat runs [bat --paging=never --style=plain --] under zsh and [bat --paging=never --] under fish` |
| each mutation restored afterwards | byte-identical, checked with `diff -q`, and the seven guards green again |
| the shape of the change (S7) | `git diff --numstat` on `config.fish`: **213 added, 0 removed**, one hunk (`@@ -218,0 +219,213 @@`); `.zshrc` carries no diff at all; both sentinels still on lines 121 and 217 |
| `gofmt -l ./internal ./cmd` | no output |
| `go vet ./...`, `go build ./...` | exit 0, both |
| the full suite, unfiltered, on this tree (`go test ./... -count=1`) | **the four Ghostty failures only** -- `TestStepInstallTerminalGhosttyReportsAFailedDownload`, `TestStepInstallTerminalGhosttyRunsTheDownloadItFetches`, `TestInteractiveGhosttyScriptFailsWhenTheDownloadFails`, `TestInteractiveGhosttyScriptDownloadsThenRuns` -- the same four this file has recorded since stage 1 and `theme-survives-install.md` records beside them; `cmd/dotfiles`, `internal/release`, `internal/system` and `internal/tui/trainer` all pass |

### Log

**L11.** **A guard that reads the wrong thing, and the first one to catch it was itself.**
The first version of the reader found functions by a closing brace in column 0. zsh's
`source_first_existing` (`:381`) closes with an indented `    }` (`:389`), so `cat` was
swallowed into the function above it and the reader found **8 of the 12** functions the
file declares. The guards then failed complaining about *zsh* while the change was in
fish -- a failure for the wrong reason, which is worse than no failure. The reader now
reads one function at a time, counting depth from that function's own declaration.

**L12.** **The same class of mistake, twice in one stage.** The message guard collected
every quoted string on a line that mentions the error stream, and zsh packs more than a
message onto one line: `esac || { rm -f "$archive"; print -u2 "compress: failed to
create $archive"; return 1 }` put the `rm` target into the comparison, and only the fish
side lacked it. The guard now reads what the error verb is given -- after `print -u2`,
before `>&2` -- and the sets match. L4 was a search that missed; these two were readers
that over-read. Both were found by the guard failing, which is the point of it failing.

**L13.** **The comparison is over policy, not text**, and that is a decision rather than
an accident of implementation: fish cannot be written as zsh, so a guard over literal
text would either forbid the translation or accept a fish body that says something else.
What S3 and S4 claim is the tool and the flags per format and the message a person
reads, so that is what is compared -- the words that carry no policy (`$`-bearing words,
redirections) are dropped before comparing.

**L14.** **A probe can be wrong while the code is right.** `extract weird.bin` was
expected to say *unsupported format* and said *no such file*, because the probe never
created the file: the check for a file comes before the check for a format, in zsh and
therefore in fish. The branch was verified afterwards with a file that exists. The
correct-looking output was the probe's fault, not the function's, and nothing was
changed in response.

**L15.** The four functions are declared outside the generated block and before the
drop-in loader, so a user's drop-in in `~/.config/fish/dotfiles.d/` still overrides them
and the artifact guard, which replaces only the bytes between the sentinels, is not
touched by this stage.

**L16.** Stage 3 was merged as `acad575` (`feat(fish): the four commands zsh declares,
so an archive is unpacked the same way`, PR #325) with all eleven checks green, including
the `macOS smoke test` that had caught the stage-2 capacity defect. The merge job refused
to run unless the PR's head was the commit this file was verified on, before and after
waiting for the checks. This record is written in stage 4's branch, which is branched from
the merge, because a merge's own hash cannot be in the branch it merges.

## Stage 4 — the hooks and the secrets

The mandate is this file's own stage 4: *"the sops/age secrets, the venv activation on
`cd`, and the project-task announcement"*.

### What was measured before writing anything

Read out of `dotfiles-zsh/.zshrc`; the line numbers are that file's.

| what | zsh | what it does |
|------|-----|--------------|
| `_dotfiles_activate_venv` | `:665-704` | hooked to `chpwd`: walks the current directory and, **only while inside a repository**, its ancestors up to the root; candidates `.venv`, `venv`, `.env`, requiring the environment's activation script to be a file; tracks what it activated in `_DOTFILES_VENV` so an environment the user activated by hand is never deactivated, does nothing when the directory is inside the project it already activated, and prints `venv: <name>` when it activates |
| `_dotfiles_show_tasks` | `:708-729` | hooked to `chpwd`: silent in `$HOME` and with `DOTFILES_NO_TASKS=1`; `just --list` for a `justfile`, otherwise `make` targets read out of a `Makefile` **without executing it**, `package.json` scripts through `jq` printed as `npm run <name>`, and `task --list` for a `Taskfile.yml`; each list capped at 20 lines |
| `_dotfiles_secrets_ready` | `:777-790` | the gate every secrets command starts with: names `sops` when it is missing, and the key file with the `age-keygen` command that creates it when the key is missing |
| `env-edit [file]` | `:792-804` | `sops <file>`, so the plaintext never reaches the disk |
| `env-load [file] [.env]` | `:806-838` | refuses unless git **ignores** the destination, refuses an unwritable one, removes the destination when decryption fails, and reports `secrets: wrote <dst>` |
| `env-export [file]` | `:840-850` | `sops -d --output-type dotenv` prefixed with `export `, writing nothing |

- **The two hooks are `chpwd`, and the third `add-zsh-hook` is not a hook.** `grep -n
  add-zsh-hook` returns three lines, but `:364` is the `autoload` that brings the function
  into the shell. Counting the matches would have claimed a third hook that does not exist.
- **fish's primitives were measured, not assumed** (`fish --no-config`): a handler
  declared `--on-variable PWD` does **not** fire at startup, fires on every directory
  change, sees the **new** directory, and what it sets with `set -gx` survives the
  handler -- `chpwd`'s behaviour under another name. `path dirname` walks the ancestors the
  way `${dir:h}` does and stops where `.git` is.
- **The virtual environment is activated through the script fish can source.**
  `python3 -m venv` writes four activation scripts, and sourcing `bin/activate.fish` sets
  `VIRTUAL_ENV`, prepends the environment's `bin` to `PATH` and defines `deactivate`,
  which puts both back (measured). The POSIX `bin/activate` beside it is not fish and
  cannot be sourced from fish at all.
- **What this host cannot verify by running it**: `just` and `task` are not installed here,
  so those two branches of the task announcement are held by the guard's comparison with
  `.zshrc` and not by a run, and the proof table says so. `jq`, `make`, `git`, `sops` and
  `age` are present, so the rest of the stage is exercised for real, secrets included.

### Specs

S1. fish declares the same three secrets commands under the same names -- `env-edit`,
    `env-load`, `env-export` -- with the same defaults (`DOTFILES_SOPS_FILE=secrets.env`,
    and `SOPS_AGE_KEY_FILE` defaulting to `~/.config/sops/age/keys.txt`), and each one
    starts with the same readiness check, so a missing tool or key is reported once, with
    the command that fixes it.
S2. `env-load` refuses to write a destination git does not ignore, refuses an unwritable
    one, removes the destination when decryption fails, and reports `secrets: wrote <dst>`
    on success; `env-export` prints `export `-prefixed dotenv lines and writes nothing.
S3. fish activates a project's environment when the directory changes and leaves it on the
    way out: only an environment this hook activated is ever deactivated, tracked under the
    same name (`_DOTFILES_VENV`), so one the user activated by hand is never touched;
    moving inside the project it already activated neither re-activates nor re-announces
    anything; the candidates are the same three, and an environment counts as present only
    when the activation script fish can source is a file in it.
S4. The search is bounded the same way: the current directory, and its ancestors **only
    while inside a repository**, stopping at the root -- so a stray environment in `/tmp`
    or in `$HOME` cannot capture unrelated projects; with no repository, only the current
    directory is searched.
S5. fish announces a project's tasks the way zsh does: silent in `$HOME`, skipped with
    `DOTFILES_NO_TASKS=1`, `just --list` for a `justfile` when `just` resolves, otherwise
    `make` targets read from a `Makefile` without executing it, `package.json` scripts
    through `jq`, and `task --list` for a `Taskfile.yml`; each list capped at 20 lines.
S6. The messages a person reads are the same strings in both shells, so a mistake reads the
    same whichever shell is running it.
S7. Nothing outside these two hooks and three commands moves: the generated theme block,
    stage 1's aliases, `EZA_ICON_SPACING`, stage 3's four commands and the drop-in loader
    stay exactly where they are.

### Tasks

T1. Declare the two hooks and the three secrets commands in `config.fish`, outside the
    generated block and before the drop-in loader.
T2. Guard it: the names, the defaults, the candidates, the repository boundary, the guards
    on the tools and the messages are compared against `.zshrc`.
T3. Prove the hooks by running them: a repository with a virtual environment (activation on
    entering, deactivation on leaving, no re-activation inside the project, and a stray
    environment in a directory outside any repository ignored) and a project that declares
    tasks.
T4. Prove the secrets commands by running what this host can: a real age key in a scratch
    directory, a real encrypted file, `env-export` and `env-load` against it, and every
    refusal on the way.

### The proof (stage 4)

| what | value |
|------|-------|
| the seven guards against `config.fish` as it stands in `acad575` | **all seven fail**, each naming what fish lacks -- `zsh runs [_dotfiles_activate_venv _dotfiles_show_tasks] when the directory changes and fish runs []`, `declares no env-load, so fish is missing a hook or a command zsh has` -- and the zsh side read correctly (37 lines for the venv hook, 19 for the tasks hook, 26 for `env-load`), so no guard complained about zsh while the change was in fish |
| the same seven after the change | pass; `fish -n` accepts the file |
| the venv hook, run in a real fish | the five rules: activates from an ancestor inside the repository; does not re-activate or re-announce while moving inside the project; leaves the environment on the way out; **a stray `.venv` beside a subdirectory does not capture it** (the rule the bounded search exists for); and an environment activated by hand survives a directory change (`_DOTFILES_VENV` empty, `VIRTUAL_ENV` unchanged) |
| the task hook, run in a real fish | `  make build` and `  make test` from a `Makefile` -- listed by the awk pass, so nothing in the file ran -- and `  npm run build` / `  npm run dev` from `package.json` through `jq`; silent in the same directory that had announced them once `DOTFILES_NO_TASKS=1` is set, and silent in `$HOME` |
| what this host cannot run | `just` and `task` are not installed, so those two branches are held by the comparison with `.zshrc` and not by a run; the guard names them either way |
| the secrets, run against a real age key and a real encrypted file | `sops -e -i` left **0** occurrences of the plaintext, header `ENC[AES256_GCM,…]`; `env-export` printed `export API_KEY=…` and `export OTHER=two`; `env-load` refused without a `.gitignore` (status 1, no file created) and then wrote the plaintext with `secrets: wrote out.env`; a corrupt file failed with `secrets: could not decrypt bad.env` and **no destination left**; `env-edit` with `EDITOR=true` decrypted and re-encrypted the same file |
| the refusal paths, one by one | missing file -> `secrets: no such file: …`; no age key -> `secrets: no age key at …` **and** `  create one with: mkdir -p … && age-keygen -o …`; no `sops` on `PATH` -> `secrets: sops is not installed`; a destination git does not ignore -> the two-sentence refusal, before anything is written |
| why the removal after a failed decrypt is not ceremony | measured on its own: `sops` exits **100** on a file it cannot read and the redirect has already created the destination, so without the removal a failed decrypt leaves an empty file that looks like a secrets file |
| the seven mutations | each fires only the guard written for it: the hook stops being `--on-variable PWD`; the candidate list loses one; `activate.fish` becomes the POSIX `activate`; the `git check-ignore` refusal goes; a message changes; `env-load` loses the readiness check; the `Taskfile` branch goes. Each restored byte-identical, and the seven guards green again |
| the shape of the change (S7) | `git diff --numstat`: **199 added, 0 removed**, one hunk (`@@ -431,0 +432,199 @@`), after stage 3's `serve` and before the drop-in loader; `.zshrc` carries no diff at all |
| `gofmt -l ./internal ./cmd`, `go vet ./...`, `go build ./...` | no output, exit 0 and exit 0 |
| the full suite, unfiltered, on this tree (`go test ./... -count=1`) | **the four Ghostty failures only** -- the same four this file has recorded since stage 1 and `theme-survives-install.md` records beside them, and the same four every run in this repository produces; `cmd/dotfiles`, `internal/release`, `internal/system` and `internal/tui/trainer` all pass |

### Log

**L17.** **Two readers that took in more than they were reading about, the third and
fourth of this session.** The candidate list came back as `[.venv venv .env;]` because
zsh writes `for candidate in .venv venv .env; do` and the semicolon arrives glued to the
last candidate, so a break on the word `";"` never fired. And the sops comparison came
back as `if ! sops -d then` against `if not sops -d`, because it compared the whole line,
keywords included, instead of the invocation. Both were the guard's own defect: fish was
implemented correctly and the guard said otherwise.

**L18.** **The fifth one produced the rule.** Reading the `git` invocations out of
`env-load` returned a phantom `git does` -- from the message `secrets: git does not ignore
$dst`, read as code. The same class had now appeared four times, in four different
readers, so this one was fixed at the root rather than at the call site: quoted strings
are stripped before any line is scanned for a command, because a message that names a
command is not that command being run.

**L19.** **Two probe defects and one suspicion that did not survive measurement.** The
first secrets run proved almost nothing: `age-keygen` prints `Public key: age1…` on
**stderr** with a capital P, the pattern looked for a lowercase `public key:` in the
combined output, so the public key was empty, `.sops.yaml` had `age: ` and the file was
never encrypted -- steps 1 to 3 then exercised a plaintext file. The same run restored
`PATH` to `/usr/bin:/bin`, where `sops` is not installed, so `env-edit` reported the wrong
refusal for the wrong reason. Both were the probe's, and both were re-run properly rather
than reported. And two `warning:` lines fish printed around a failed redirect in that run
**did not reappear** in a clean one or in isolation, with zsh beside it producing the same
result: recorded here as a suspicion that measurement did not support, and nothing was
changed in response to it.

**L20.** A small process mistake worth one line: three assertions in the probe were
written with `and` / `or`, which are fish keywords, inside a bash command substitution.
They failed with *command not found* and the two checks they held were run again. The
shell a line is written for is not the shell a line is written in.

**L21.** **No document changed, and that is a decision rather than an omission.** zsh's
venv hook and secrets commands are documented nowhere -- the only mention of `sops` and
`age` in `docs/` is the Brewfile row in `docs/tools.md`, which lists formulae rather than
shell commands. Stage 3 did change `docs/tools.md`, because its `cat` row named an alias
where the shell declares a function; nothing here has an equivalent row to correct. The
introductory sentence of that same table says what the zsh configuration starts, which is
still true, so it is left as it is.

## Remaining

Stage 4 is written and in review on this branch: the hooks and the secrets. With it the
mandate that opened this file is whole. The next piece of work is a different one -- the
installer's background becomes a slow-drifting gradient, decided separately and recorded
in memory rather than here.
