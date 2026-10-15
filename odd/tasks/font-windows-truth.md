# The font, told the truth on Windows

## Goal

The installer already puts Meslo on the machine and already verifies more than it
attempts. This is about the part of that path which is still guesswork: **which Windows
account it is configuring**, and **what Windows Terminal actually ends up reading**.

## Evidence, measured on the user's own machine (read-only)

| what | what it says |
|---|---|
| `cmd.exe /c echo %USERPROFILE%` | `C:\Users\t749597` — the robust answer exists and works |
| `$USER` inside WSL | `alberto` |
| `$USERNAME` inside WSL | **not set at all** |
| `/mnt/c/Users` | `defaultuser0`, `ES99P6NCS5A7A0H$`, `t143298o`, `t749597` |
| Windows Terminal | the **Store** package, `Microsoft.WindowsTerminal_8wekyb3d8bbwe` |
| `profiles.defaults.font.face` | `MesloLGS NF Herdr` — **the font is applied, and it works** |
| `profiles.defaults.colorScheme` | **`None`** |
| `schemes` | **`[]` — zero** |
| the user font store | 30 files, including `MesloLGS NF Herdr` ×4 **and** `MesloLGS NF` ×4 |
| HKCU font registry | `MesloLGS NF Herdr Regular/Bold/Italic/Bold Italic` + `JetBrains Mono Herdr` |

## The defects

### D1 — `windowsSystemUserDir` does not know `defaultuser0`

`windowsProfileFromMount` skips `public`, `default`, `default user`, `all users` and
`defaultapppool`, and **`defaultuser0` is none of those**. It exists on this machine, it is
a Windows system account created by setup, and it is a candidate. On a machine where it is
the only non-excluded directory — or where it sorts first — the installer configures the
wrong account. Its own doc comment says writing into one of these "would either fail or
configure the wrong account", which is exactly what the missing name does.

### D2 — The mount lookup can never match, so it is not a fallback

The mount fallback compares a profile directory name against `$USER` and `$USERNAME`. In
WSL, `$USER` is the **Linux** account (`alberto`) and `$USERNAME` is **unset**, so neither
can equal `t749597`. With more than one candidate the lookup therefore returns "several
Windows user profiles found" and refuses. It works today only because the primary resolver
answers first — so what looks like a fallback is, in the state it is written for, a refusal.

### D3 — CORRECTED: the offer was reachable, and the report lied

**This section was wrong when it was written, and the correction matters more than the
original claim.** It said the interactive path offered and never wrote, because
`applyWindowsTerminalOffer` appeared to be called only from the `--non-interactive` branch.
That was concluded from a **file-scoped** grep of `wslterminal.go`, which cannot see
`update.go`. Every caller, over the whole package:

```
update.go:404        applyWindowsTerminalOffer(offer, false, wslStepID)   <- the "no"
update.go:416        applyWindowsTerminalOffer(offer, true,  wslStepID)   <- the "yes"
update.go:1848       applyWSLTerminalOfferCmd(true)                       <- the y key writes
update.go:1850       applyWSLTerminalOfferCmd(false)                      <- the n key respects
update.go:3078       wslTerminalOfferCmd()                                <- the step is intercepted
wslterminal.go:1210  applyWindowsTerminalOffer(offer, true, wslStepID)    <- the non-interactive path
```

Commit `5212857 feat(wsl): the renderer offer reaches a screen` wired all of it, with three
guards: `TestInteractiveWSLStepShowsTheOfferOnTheInstallingScreen`, `...YesWritesTheFile`,
`...RefusalLeavesTheFileUntouched`. So a person running the installer sees the offer on the
installing screen, `y` writes, and `n` does not.

**The defect that was real under it**: `applyWindowsTerminalOffer` built its notice from the
settings it had **sent**, never from the file it had **written**, so a landing that came out
partial printed a success over it. Measured on the real machine as `colorScheme: None` with
the face applied — which the offer being unaccepted explains, and the checkbox explains too.
It is fixed: the write is read back with the same readers, each setting is reported, and a
partial landing is a shortfall carrying `wrote=true` rather than a checkmark.

**The lesson is mine and it is the same one this front keeps producing**: the scope of a
search decides the conclusion it can reach. A grep of one file answered "exactly once" about
a function with five callers.

### D4 — Termux never got Meslo, and the interface says it did

`model.go` tells a person, in five places, that this installs **Meslo**: the wizard row
("Yes, install Meslo Nerd Font"), the requirement line ("Meslo Nerd Font is required for
icons and glyphs"), the menu row ("Install Meslo Nerd Font", "Installs the Meslo Nerd Font
for icons") and the summary ("Font: Meslo Nerd Font"). On Linux, macOS and Windows that is a
fair shorthand -- all three end up with a family named `MesloLGS NF`, verified here by
reading the `name` table of the installed faces:

| file | family it declares |
|---|---|
| `MesloLGS NF Herdr Regular.ttf` | `MesloLGS NF Herdr` |
| `MesloLGS NF Regular.ttf` | `MesloLGS NF` |
| `JetBrainsMonoHerdr-Regular.ttf` | `JetBrains Mono Herdr` |

**In Termux it is simply false.** `termuxFontFace` is `JetBrainsMono Nerd Font` and
`termuxFontURL` downloads that font, so the step's own log names JetBrains Mono while the
menu a person pressed said Meslo. The code says so in a comment and calls it "the rest of
issue #269, outside this change's surface" -- and #269 was closed by #276 without it.

This matters more than a wrong label: **the decision recorded in #265 was that the font is
Meslo**, for every terminal. Termux never received it.

### D5 — The face is named in more places than it is decided

`Meslo` appears in five interface strings, `MesloLGS NF` in one constant, `MesloLGS NF
Herdr` in another, `MesloLGSNerdFont-` as a file prefix, and `JetBrainsMono Nerd Font` in
Termux. Nothing derives one from the other, which is why they disagree.



### D6 — The Linux presence check asks for a family the archive does not declare

Found while verifying a URL, and it is the biggest defect on this front.

`installer.go:5538` and `:5604` both call `installedLinuxFontFamily(windowsTerminalFontFace)`,
so fontconfig is asked whether the machine has the family `MesloLGS NF`. The archive the
installer pins for Linux is `Meslo.zip` v3.3.0, and **the faces inside it declare
`MesloLGS Nerd Font`** -- verified by reading the `name` table of the face fetched from the
repository at the pinned tag, through the API with its blob sha.

So `known && resolved` is false on a machine that has the font installed correctly, the step
concludes the family is absent, and **it downloads 108 MB again on every run** -- which is
precisely the defect #276 was opened to close. #276 asked fontconfig *before* downloading, and
the question it asked was the wrong one, so the guard it added cannot fire.

Confirmed locally: on this machine `fc-list : family` has no Meslo entry and
`~/.local/share/fonts` holds no Meslo file, consistent with the Linux step never having run
here. The same constant is used for the Windows registration, where the name that matters is
the one herdr registers.

The names in play, all measured:

| where | family |
|---|---|
| the pinned Linux/Termux archive (v3.3.0) | `MesloLGS Nerd Font` |
| the macOS cask `font-meslo-lg-nerd-font` | to be checked in T8 |
| what herdr registers on Windows | `MesloLGS NF Herdr` |
| what the installer's HKCU registration writes | `MesloLGS NF` |

**Four names for one font, and the check typed one of them.**

### D6, completed — one wrong constant, and everything downstream obeyed it

The full shape, found by the worker who was fixing it: it is not only the Linux check.

`windowsTerminalFontFace = "MesloLGS NF"` is a hand-typed name, and the faces in every
artifact this installer fetches declare **`MesloLGS Nerd Font`**:

| artifact | what it declares | how it was checked |
|---|---|---|
| the pinned Linux/Termux archive, v3.3.0 | `MesloLGS Nerd Font` | `name` table of the face at the pinned tag |
| the macOS cask `font-meslo-lg-nerd-font` v3.5.1 | `MesloLGS Nerd Font` | blob sha through the API, then `fc-scan` |
| the Termux face (same repository) | `MesloLGS Nerd Font` | `fc-scan` on the fetched blob |
| what herdr registers on Windows | `MesloLGS NF Herdr` | the installed face's `name` table |

Four consequences, all real, and only the first was in this document before:

1. **The Linux presence check never resolved**, so the step redownloaded 108 MB every run --
   the defect #276 was opened to close, with a guard that could not fire.
2. **The post-extraction verification failed**, so the Linux font step was **broken, not
   merely slow**: it reported an error on a machine that had the font installed correctly.
3. **The four shipped configurations** (`alacritty.toml`, `.wezterm.lua`,
   `dotfiles-ghostty/config`, `dotfiles-kitty/kitty.conf`) named a family **no installed face
   declares**, so a fresh Linux or macOS install would have had every terminal fall back to
   its default font -- silently, with no icons and the wrong letterforms.
4. **The guard written to prevent exactly this**,
   `TestEveryTerminalConfigNamesTheFamilyTheArchiveProvides`, whose comment says every family
   a config names "must be the one the archive provides", compared against that same wrong
   constant. It was doing its job faithfully against a false premise.

**And the commit message that named the wrong family**: `d24a52f feat(font): install Meslo,
and name the family the machine actually resolves` claims the v3.3.0 archive's public family
is `MesloLGS NF`. It is not. A commit message is history and cannot be rewritten, so the
correction lives here, in the project's record of this front, for whoever reads the log next
and would otherwise trust it.

**The lesson**: one name typed in one place and a different name living in the artifact is
the whole family of defects on this front. The fix is one constant read from the thing that
decides it, every other name derived from it, and a guard that asserts two things agree
rather than one that asserts a literal.

## The break (corrected: there was no break in reachability)

The writer is correct and the chain is broken in one place. The call graph, read from the
source rather than inferred:

```
wsl.go:76            stepInstallWSLConfig(m)                            <- the installer step
  wsl.go:114           configureWindowsTerminalRenderer(repoDir, profileDir)   <- its ONLY caller
  wslterminal.go:1167  configureWindowsTerminalRenderer(...)
    wslterminal.go:1191  if !nonInteractiveMode {
    wslterminal.go:1196      "Windows Terminal was left unchanged: ... only after an explicit yes."
    wslterminal.go:1197      return                       <- THE INTERACTIVE PATH ENDS HERE
    }
    wslterminal.go:1204  installAndVerifyWindowsFont(profileDir)     <- non-interactive only
    wslterminal.go:1210  applyWindowsTerminalOffer(offer, true, ...) <- non-interactive only
```

**`applyWindowsTerminalOffer` is called exactly once, at line 1210, inside the
`--non-interactive` branch.** The interactive branch prints the offer and returns, saying the
file "was written only after an explicit yes" — and **nothing anywhere supplies that yes**.
`wslTerminalInteractiveOffer` is called from `update.go:387` to put the offer on a screen;
there is no path from that screen or from any keypress to `applyWindowsTerminalOffer`.

So the interactive default — the one a person actually runs — **offers and never writes**.
The real machine agrees: `font.face` is set (a `--non-interactive` run, or herdr, put it
there) and `colorScheme` is `None`, because the interactive path never reaches the writer.

This is the third path of this family in this project: the offer that never reached a
screen, the step a test reached directly while the interface could not, and now a writer
whose only caller is a branch a user does not take. **A green suite cannot see any of them**,
because a test that calls the writer directly proves the writer, not the reachability.

T3 is therefore: **make the offer takeable from where it is shown, or stop showing it as if
it were**, and have the step report which of the two landed.

## What is already right, and must stay right

- **Verification gates the write.** The family is read back from the slot the terminal will
  read, and the report is the verification rather than the attempt.
- **The Store package is found**, by glob, without hard-coding a version.
- **Two settings files are refused, both named**, rather than one being picked.
- **The machine's own face wins** over the archive's family name — `MesloLGS NF Herdr` is
  what herdr registers and what the terminal reads.

## Tasks

- [ ] **T1 — Ask Windows who the user is, and check the answer.** Resolve the profile from
  `%USERPROFILE%` through `windowsEnvVar`, and require the directory to exist on the mount
  before trusting it. `defaultuser0` joins the system-account list. The mount scan stays as
  a last resort, and a single leftover candidate is a refusal rather than an answer.
- [ ] **T2 — Prove T1 against the real machine.** Each step: the real answer
  (`t749597`), a mount holding only `defaultuser0` (refused, named), a wrong
  `%USERPROFILE%` (falls through to the mount), and two real profiles (refused, both named).
- [x] **T3 (done, corrected) — Report the write from the file, not from the bytes.** The
  premise of this task was wrong (see D3): the offer was already reachable. What was missing
  is the second clause: the report described what was sent and could print a success over a
  partial landing. `applyWindowsTerminalOffer` reads the file back now, reports each setting,
  and returns a shortfall for anything that did not land, on both paths.

- [ ] **T4 — Read the result back from the file Windows reads.** After the write, parse the
  settings file it edited and report the face and the scheme it now holds, rather than the
  bytes that were sent.
- [ ] **T5 — The local proof.** Run the whole font path against this machine's real
  Windows, and report what changed: the face, the scheme, and the store.
- [x] **T8 (done) — One family constant decides every name.** See D6 above.
- [x] **T6 (done) — Termux gets the font the decision names, or the label stops lying.** Prefer
  the first: the archive this installer already pins for Linux holds `MesloLGS NF` faces, so
  Termux can be given one of them without a new download source. If a reason to keep
  JetBrains Mono on Termux is found, then the wizard row, the requirement line, the menu row
  and the summary must name the face per platform instead of saying Meslo. **Whichever is
  chosen, the interface stops claiming something the machine did not receive.** If the
  choice is genuinely open, ask the user rather than deciding for them.
- [ ] **T7 — One place decides the face.** Every interface string that names a font derives
  it from the constant that installs it, so the label cannot drift from the machine again.

## After this front: the pin moved

D6 measured the archive this installer pinned for Linux at the time -- `Meslo.zip` v3.3.0 --
and recorded that its faces declare `MesloLGS Nerd Font`. The measurements above are left as
they were taken. This is what changed since, so that a reader does not take them for the
current state.

**The pin is `Meslo.zip` v3.5.1 now (issue #313), and the facts D6 rested on were re-measured
at the new tag rather than assumed to carry over:**

| what | at v3.3.0 | at v3.5.1 |
|---|---|---|
| the family the faces declare (`name` id 1) | `MesloLGS Nerd Font` | `MesloLGS Nerd Font` -- unchanged, so the presence check and the post-extraction verification keep working |
| `dev-yaml`, `U+E8EB` | **absent** | **present**, glyph id 4074 |
| the archive's declared size | 108,281,715 bytes | 116,387,026 bytes |

The codepoint is why this is a fix rather than housekeeping: `eza` draws a `.yaml` file with
`U+E8EB`, so on v3.3.0 every `.yaml` in a listing came out as a blank box on Linux, WSL and
Termux while macOS -- whose cask installs v3.5.1 -- drew it correctly. That is also what makes
the version a floor rather than a preference: `U+F086F` and `U+F15DE` are present at both tags
and `U+E8EB` only at the later one, so lowering the URL restores the blank box. The archive's
`glyphnames.json` at each tag is what settles it, and is what a future bump is checked against.
