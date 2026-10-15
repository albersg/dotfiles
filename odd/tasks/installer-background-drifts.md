# The installer's background drifts

Feature: the installer paints a background derived from the theme behind **every** screen it
can show, and that background drifts slowly while the animation gate is on.
Branch: `feat/installer-drift-background`, cut from `6515ee9` (the merge of the fish hooks).

## Specs

**S1.** The background is dynamic in **every** tab and every place inside the installer. Quoted
from the request: *"el background del installer, en todas las pestañas y lugares dentro del
installer, sea un fondo dinamico"* -- all of them, not the welcome screen.

Its observable form: for **every** screen in the shared matrix, the render with the drift on
differs from the same model with the drift off -- no screen is left out, and the difference is
caused by the background and by nothing else.

**S2.** The shape is the one chosen: a gradient **derived from the theme** that drifts **slowly**,
on the frame cadence that already exists. `animTicksPerSecond = 8` (`anim.go:44`, one tick every
125 ms) is the drift's rate; no second timer, no second cadence, and the drift advances on the
tick the companion already rides.

**S3.** The drift is a **pure function of the model's frame counter**. The render path reads
neither the clock nor the environment -- the rule this package already states for the animation
gate (*"the render path itself never reads the environment, so a render stays pure"*,
`anim.go:110-112`) and the same reason the greeting is pinned to `CreatedAt` instead of
`time.Now` (*"a greeting read from the clock while drawing would change the bytes when nothing
else did, and no snapshot could pin it"*, `view.go`). Two renders at the same counter are
byte-identical; at counters `n` and `n+k` the render **differs**, so a static gradient fails the
guard.

**S4.** Off means off, in two forms. In the file: with the drift off the render does not change
with the frame counter at all -- counters `n` and `n+k` are byte-identical. At the fleet level,
and this is the one that matters: **every existing frame comparison in the package passes
unchanged** -- 227 calls to `View()` across 20 test files -- which is the repository's own proof
that a drift-off run produces today's bytes.

The user-facing switch is the one that exists, `--no-anim` / `DOTFILES_ANIM=0`. The animation gate
itself (`animationGate(stdout)`, `anim.go:113`) is what it always was -- that switch, `TERM=dumb`,
and a character device; it has never asked about colour, and L9 records the parent's error in
saying it did. The drift's own answer therefore adds the terminal's colour profile, taken once at
build beside it, exactly as the sprite asks for true colour (`pixelSpriteAllowed`,
`companion.go:2208-2212`): without 24-bit colour the ramp would be approximated or dropped, and a
background that is not the one the theme chose is worse than the theme's own. Answered once at
build is what keeps the render pure and lets the painter trust the field alone.

**S5.** Only colour changes: never text, never geometry. For any screen and any counter, with
**everything identical except the drift** -- the same model, the same animation gate, the same
sprite, the same counter -- the ANSI-stripped renders are identical: same rows, same order, same
number, and the same width per row. Comparing the animation gate's on and off states instead
would measure the creature and the live panel's text rather than the background; see L6.

**S6.** **One** paint point, not twenty. Every screen goes through it, so a screen added later
gets the background without doing anything and no screen can opt out.

**S7.** The switches stay independent, as they are today: the animation, the mouse and the sprite
are three separate answers. The background does not turn the creature off, and the sprite's
answer does not switch the background: with `PixelSprite` off and the animation on, the drift
still makes the render differ.

**S8.** The help text and the documentation that **enumerate what `--no-anim` stops** name the
background too: `--no-anim`'s help at `cmd/dotfiles/main.go:421` (and `:399` if that line
enumerates), and the switch table in `docs/tui-installer.md:1269`.

## Tasks

| ID | Specs | What | Route | Commit |
|----|-------|------|-------|--------|
| T1 | -- | This file: the specs above, the log, and the measurements the specs rest on | parent | (the branch's first) |
| T2 | S1-S7 | The guard, written to fail against `view.go` and `anim.go` as they stand in `6515ee9` | parent | 1 |
| T3 | S1-S7 | The background: the paint point, the gradient from the theme's colours, the drift on the frame counter | parent | 1 |
| T4 | S4-S7 | Mutations, one per rule, each firing only its own guard and restored byte-identical | parent | 2 |
| T5 | S8 | `CHANGELOG.md`, the switch table in `docs/tui-installer.md`, `--no-anim`'s help | parent | 2 |
| T6 | all | The full unfiltered suite, the commit, the PR, and the merge with the commit safeguard | parent | 3 |

## Log

**L1** (the request, verbatim): "podriamos ir trabjando tambien que, el background del installer,
en todas las pestañas y lugares dentro del installer, sea un fondo dinamico? se podria hacer?"

**L2.** Answered with the reconnaissance and then decided together: **a gradient derived from the
theme with a slow drift**, on the frame tick that already exists, and **after** stage 4 of the
fish work rather than beside it. Recorded then in memory under
`dotfiles/installer/dynamic-background`; this file is where the decision lives durably.

**L3.** What was measured before writing any of these specs, so that none of them rests on an
assumption:

| what | measurement |
|------|-------------|
| the paint point | `View()` (`view.go:987-1114`) is a `switch` over every screen writing into one `strings.Builder`, and then a **single tail**: `content := s.String()`, the leader-mode replacement, and one `return paddedStyle.Render(content)` with `Padding(viewPaddingRows, viewPaddingCols, 0, viewPaddingCols)`. One assembly point already exists; the forty cases are not touched |
| the chrome | `frame()` (`:367`) through `frameWithPanels()` (`:381-471`) place the body, the panels, the companion's reserved block and the end-of-run burst, and already yield decorations when the frame has no room -- the precedent the background yields by too |
| the cadence | `animTicksPerSecond = 8` (`anim.go:44`), `animTickInterval = time.Second / 8` (`:51`), `animTickMsg` (`:86`), `animTickCmd()` (`:92`), and two ticks are one celebration frame (`:193`) |
| the gate | `animationGate(stdout *os.File) bool` (`anim.go:113`), reading `DOTFILES_ANIM` once at model build and keeping the answer on the model; the sprite's separate gate is `pixelSpriteAllowed` (`companion.go:2208`) and the interval's is `envSprite` |
| the colours | `styles.go`: `Background lipgloss.AdaptiveColor` (`:35`, `:118`) and `Accent` (`:40`, `:123`), exported as package variables at `:195` and `:200`; the installer already adapts to the terminal with `lipgloss.HasDarkBackground()` (`model.go:489`) and AltScreen is already on (`cmd/dotfiles/main.go:189`) |
| what would have to change | **20 test files render frames** through `.View()`, in **227 calls**; the design updates none of them, because with the gate off the bytes are today's (S4) |
| what documents it | `docs/tui-installer.md:1260-1271`, whose table says `DOTFILES_ANIM=0, --no-anim` stops *"the frame tick and the tip rotation"*, and the help at `cmd/dotfiles/main.go:399` and `:421` |

**L4.** **The reconnaissance's own number was wrong and the correction is the interesting part.**
It counted twelve test files comparing frames; measuring says twenty files and 227 calls to
`View()`. That number would have made the change look cheaper than it is -- except that the
design makes it irrelevant: the gate's answer is read once at model build and the off case
produces today's bytes, so a hundred more call sites would cost nothing. The mistake did not
survive contact with a real count and nothing was built on it.

**L5.** One consequence to settle while implementing rather than late, named here so it is not a
surprise: a test that renders with the gate **on** compares bytes that now carry the background.
The render is pure (S3) and its plain text is unchanged (S5), so such a test either pins the new
bytes or builds its model with the background off -- which is a decision for each of them, made
after measuring which are affected, and not a reason to weaken S4.

**L6.** **The guard came back with three findings and two of them were defects in these specs,
not in the code.** (1) S5's literal form -- "the stripped render with the background on equals
the one with it off" -- is unsatisfiable, because `Animating` is not a background switch: it also
draws the companion (`companion.go:2588`), moves the panel-summary row down, changes the live
panel's text (`metrics.go:180`) and the installing progress highlight (`view.go:3213`, `▒` against
`█`). The colour profile is no better as a reference: rendering the same model with and without
it changes that panel's **text**, not only its colours. (2) "Carries any background SGR" is not a
signature only the drift can produce: under true colour the existing chrome already fills with
`48;2`, and under the ambient profile the pixel sprite already carries backgrounds in **425 of the
732** shared frames -- so a background-present assertion could pass with no drift at all, and the
parent's "any background SGR" definition is only meaningful under the ambient profile. (3) S4 and
S5 cannot be RED before the feature exists: there is no background to detect and nothing for it to
change.

The consequence is a design decision rather than a workaround: **the drift gets its own answer on
the model** -- a `bool` read once at build from the same `--no-anim` / `DOTFILES_ANIM=0` gate,
exactly as `PixelSprite` is a sub-answer of the same animation with its own field. With that field
the assertions become exact and signature-free: S1 becomes "the render with the drift on differs
from the same model with it off, on every screen", S5 becomes "with everything else identical,
only the colour changes" -- satisfiable, and it was not before -- and S4 becomes "with the drift
off the render does not move with the counter", plus the fleet-level one the package already
proves by running unchanged.

**L7.** **The drift gets no user-facing switch of its own, and that is deliberate.** The table in
`docs/tui-installer.md` explains that the three switches are independent *on purpose*, so adding a
fourth would be the consistent move -- but the request did not ask for one, and what was decided
with the user is that the animation's switch stops the background. The field above is internal
structure: `--no-anim` still turns the drift off (S8) and nothing new appears in the help.

**L8.** **The implementation changed one sentence of the brief and was right to.** The parent's
instruction said the ramp's ends were "the theme's background at one end, a lightly accent-tinted
variant at the other"; the implementation made the visible band a **smooth cosine fold** of that
ramp instead, because a gradient that does not drift is not the feature, and one shifted with a
wrap has a hard seam that reads as a stripe competing with the text. The ramp's ends are still the
theme's `Background` and a 20%-accent tint, every row interpolates them, and the band scrolls
half a row per frame tick -- four rows a second at `animTicksPerSecond = 8`, so the screen takes
several seconds to cross and no single tick reads as a jump. S2 asked for a theme-derived gradient
that drifts slowly and that is what it got; the pinned ends were the parent's elaboration, not the
spec.

**L9.** **The parent's S4 named a gate that does not exist, and measuring it corrected the spec.**
S4 said the animation gate applies "no 24-bit colour". Reading it (`anim.go:113-121`) shows three
conditions and no more -- `DOTFILES_ANIM=0`, `TERM=dumb`, `isCharDevice(stdout)` -- and colour was
never one of them. The 24-bit question is asked by the **sprite's** gate,
`pixelSpriteAllowed(profile, spriteEnv)` (`companion.go:2208-2212`), which requires
`termenv.TrueColor`; that is the repository's own precedent for a colour ramp. The drift's answer
now includes it: `Drifting` is the animation gate **and** a true-colour terminal, computed once in
`NewModel`, never while drawing. Three things follow, and all three are what the design wanted: the
render path stays pure, the painter keeps trusting the field alone rather than reading the
environment, and forcing `Drifting` in the guard paints in any profile -- which is why S1's
difference and S5's identity both hold where they are measured.

### The proof

| what | value |
|------|-------|
| the seven guards against the tree before the feature existed | **five of seven red**, each naming what was owed: `Model has no forceable Drifting bool: no screen can be turned on or off for its background` (S1, S2, S3, S7), `model.go declares no Drifting bool on Model` and `the package defines no paintDriftBackground` (S6). S4 and S5 are the preserve half: green before, and required to stay green. The package **built** -- the RED was a test failure, never a build failure that would have taken the package's other tests down with it |
| the same seven after the implementation | green, `ok ... 3.7s`, with the package's existing matrix guards passing beside them |
| the fleet half of S4, measured | the whole `internal/tui` package, unfiltered: **193s and the four pre-existing Ghostty failures only** -- the 227 `View()` comparisons across 20 test files unchanged, which is what makes a drift-off run today's bytes |
| the shape of the change (S6) | `model.go` +11 (the field, and the gate it is read from once at build), `view.go` +6 (the single call at the tail), `drift.go` **117 new** (the ramp, the fold, the colour arithmetic and the painter). `paintDriftBackground` is defined once and called once, inside `Model.View`, asserted by parsing the package rather than by reading text |
| the eight mutations | M1 a second paint point -> **S6 alone**; M2 the field renamed -> S1, S2, S3, S6, S7 (the name is the contract five rules need); M3 a painter that yields on tight frames -> **S1 alone**, naming the **5 of 732** screens it skipped; M4 the frame tick stops re-arming -> **S2 alone**, on its second assertion; M5 the phase reads something outside the model -> **S3 alone**; M6 the early return removed -> S1, S4, S7 **and `TestWelcomeScreenGolden`**, the fleet half of the same rule; M7 a space appended to each row -> **S5 alone**; M8 the painter asks the sprite's question -> S2, S3, S7. Each restored byte-identical: four hashes re-checked after every one, and the guards green on the tree that remained |
| the gate the parent described wrongly | S4 said the animation gate applies "no 24-bit colour". It does not: `animationGate` (`anim.go:113-121`) checks the switch, `TERM=dumb` and a character device, and colour has never been one of them. The drift's own answer therefore adds the profile, taken once at build, the way the sprite's gate asks for true colour (`pixelSpriteAllowed`, `companion.go:2208-2212`). See L9 |
| what this host cannot show | the gradient is colour, so its evidence here is the byte difference on every screen and the first rows pasted with the ANSI visible. How it looks in a real terminal is not something this host can say: the assistant's shell has no TTY. The user's own run is the only place that can |
| the texts that enumerate what `--no-anim` stops | `cmd/dotfiles/main.go:421` and the switch table at `docs/tui-installer.md:1269`; measured first, and **no test pins either**, so both were updated in the same change as the behaviour they describe |
| `gofmt -l internal/tui`, `go vet ./internal/tui` | no output, no output |
| the full suite, unfiltered, on the final tree (T6) | **the four pre-existing Ghostty failures only**, `internal/tui` at 192s with `-count=1` and no filter; `cmd/dotfiles`, `internal/release`, `internal/system` and `internal/tui/trainer` all pass. The four are the same ones this file and `theme-survives-install.md` have recorded since stage 1 of the fish work, and the documentation this change carries -- the CHANGELOG that `internal/release` reads, the help text and the switch table -- broke nothing |

### Log

**L10.** **A job that never executed a test, and the two habits that hid it.** The first run of the
mutations came back `exit 0` in eleven seconds with `[setup failed]` on all eight. The cause was
the parent's own invocation: `go test ./internal/tui` was run **from inside** `internal/tui`, so Go
resolved the pattern against the current directory and looked for `internal/tui/internal/tui`, which
does not exist. `go build ./...` passed throughout, which is why the job looked healthy; `go build`
ignores test files, and the pattern that broke was only in the test command. On top of that, the
parent's own `grep` kept only lines starting with `ok`, `FAIL` or `--- FAIL`, and the line that
named the cause started with `#`. Both habits were already lessons in this file -- a pattern that
drifts when its directory changes, a filter that hides the reason -- and here they met in one job
and produced a green-looking result that measured nothing. The re-run used `go test .` from the
package directory, where the spelling cannot drift, and prints the raw first lines whenever a run
fails to set up. Nothing was concluded from the first job, and the tree was verified byte-identical
after every one of its eight attempts.

**L11.** **What the mutations say, and the difference between the isolating ones and the coarse
ones.** Five mutations fire one guard and nothing else (M1 on S6, M3 on S1, M4 on S2, M5 on S3, M7
on S5), and that is what proves a rule's teeth: the guard is the only thing standing between the
defect and the user. The other three fire several, for a reason worth naming rather than hiding:
M2 renames the field that five rules force, so five of them report the missing contract; M8 asks
the sprite's question, and the drift's own subject screens have the sprite off, so the two rules
that measure motion report it too. The most valuable line in the set is M6's:
`TestWelcomeScreenGolden` -- an existing frame comparison this change never touched -- fails beside
the three drift guards, which is the fleet-level half of S4 caught by the repository's own test
rather than by anything written for this feature.

**L12.** The implementation changed one sentence of the brief (a cosine fold instead of a ramp
pinned to the top and bottom edges), because a gradient that does not drift is not the feature and
one shifted with a wrap has a hard seam that reads as a stripe competing with the text; the ends
are still the theme's background and a 20%-accent tint. See L8.

**L13.** The task table planned three commits -- guard and implementation, then mutations and
documentation, then the evidence. What ships is **one**: the mutations are evidence and change no
file, and the repository's own practice for a feature is a single work unit carrying the guard, the
code, the documentation and this front together, which is what every fish stage did. The plan's
commit column is left as written; this line records what it became.

## Remaining

T6's last step is the merge, and this front cannot record it: the merge commits after this branch
is gone. As with stage 4 of the fish work (`odd/tasks/fish-parity.md`, L22), the record belongs in
whatever branch follows, and the Remaining section here is the note that says so rather than a
silent gap. The fish work this was queued behind is complete and in `main`
(`odd/tasks/fish-parity.md`, L22).
