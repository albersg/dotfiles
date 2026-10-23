# Installer redesign — the mockup's design language

The user supplied a six-panel mockup (Dashboard, Main Menu, Setup, Installing,
Utilities, Theme Selector) and asked for a large interface refactor in its style,
including every page the mockup does not show. The image lives outside the repo
and is not committed; the specs below record what it shows and the exact strings.

## Specs

S1. **The design language.** The mockup's layout is the target: bordered chrome,
    two-line rows with Nerd Font icons, a filled selection bar, cards, the setup
    table, the theme grid and the progress screen. The **theme system stays the
    one source of colour**: every surface draws its roles from the theme applied
    through the installer, the same theme the installer writes to the terminal and
    the system; no palette is hardcoded and no new theme is added. The mockup's
    violet is its illustration, not a required palette. Every colour still clears
    the existing contrast floors (body 4.5:1, the muted role on the drift, the key
    tone).
S2. **The chrome.** Every screen carries a header bar (the screen name in a
    filled chip at the left, the build label at the right) and a status bar (a
    coloured dot and a state word at the left, the key hints at the right). The
    state words are `Ready`, `Installing…`, `Preview mode`, `Done`, `Failed`.
S3. **The rows.** A list row is two lines: the icon and the title on the first,
    a muted subtitle on the second. The selected row's title line is a filled
    bar; the subtitle stays plain. Group titles carry an icon and their own
    subtitle. The Nerd Font glyphs are 1-cell; a terminal without the font must
    still draw the row (the glyph is decoration, never the label).
S4. **The main menu** shows `Start Installation` / `Install or update your
    dotfiles`, the update-channel row / `Switch between dev / stable`, `Restore
    from Back up` / `Rollback to a previous setup`, `Learn` / `Vim & Neovim,
    tools, tips...`, `Session` / `Utilities and advanced tools`, `Exit` /
    `Goodbye!`. The right panel is the selected row's title, the pet, its
    description, and the button `Press Enter to continue`.
S5. **The dashboard** is the dotfiles lockup, the tagline `Your personal
    development environment.`, four machine cards (`OS`, `Shell`, `Arch`,
    `Packages`), the greeting `Good morning, <user>!`, the environment summary,
    `Let's make this machine yours.`, and the pet.
S6. **The setup** is a table titled `Configure your environment`, subtitled
    `Each option is configured on this screen. Press Space to toggle.`, with the
    rows Platform, Shell, Oh My Zsh + Powerlevel10k, Theme, Terminal, Font,
    Multiplexer, Editor (Neovim), Existing configurations and Continue to the
    plan; each value is `[x] install`, `[ ] skip`, a chosen value, or `back up`,
    and each row carries `❯`. The right panel is the pet and the selected row's
    description.
S7. **The installing screen** is `Running installation...`, the line `Step N of
    M - <step>`, a checklist with each step's state, percent and time, a log
    panel, a progress bar (`40%`), and the footer `Installing...` / `[q] Cancel`.
S8. **The utilities** screen has the rows `System Overview`, `Diagnostics`,
    `Diagnose WSL`, `Shell Startup`, `Installed Tools`, `SSH Configuration`,
    `Network Diagnostics`, `Find Available Port`, `Clean Dev Caches`, `Back`,
    each with its subtitle, and the right panel `System Overview` with OS, Host,
    Arch, Shell, Packages, Home, Kernel, Uptime; `Resources` with CPU, Memory and
    Disk meters; and `Everything looks good!`.
S9. **The theme selector** is `Choose a theme for your installer`, `This will
    change the colors and style of the interface.`, a 2x3 grid of theme cards
    with miniature previews and the selected card marked, a tip, and the footer
    `Preview mode`.
S10. **Every other page** — the wizard pickers, the update center, the doctor,
    the Learn pages, the trainer, the backup and restore screens, the completion
    and the dead ends — follows the same grammar: the header/status chrome, the
    two-line rows, the card and its border, one footer notation.
S11. **The floor and the stability.** At 80x24 every action stays reachable and
    no region moves as the cursor moves; the wide layout may scroll its list
    instead of overflowing. Every value comes from the model or the system; no
    panel invents a fact. The 4.5:1 floor holds for every role.

The user's words, verbatim:

- "Una vez tengas todo, tengo esta imagen, para un refactor bastante grande la
  interfaz de los dotfiles. Puedes hacer algo así, y ajustar el resto de pestanas
  que no salen en la iamagen, para seguir el mismo estilo estetica y visual que
  estas pantallas, para dejarlo todo perfectamente
  /tmp/pi-clipboard-871b9484-2534-4d60-931c-26fbf88db120.png."
- "continua con la remodelizacion de la interfaz".

## Tasks

| ID | Specs | Route | Commit |
|----|-------|-------|--------|
| T1 | S1 | The theme pass: every new surface uses the applied theme's roles; the theme switch still recolours the installer and the system together; the guards measure the switch, the builder and the contrast; no hardcoded palette | (in the tree, uncommitted — the role sweep, the marker-palette guard, the filled-style contrast guards and the emblem's rebuilt ramp) |
| T2 | S2 | The frame chrome: the header bar and the status bar, the per-screen state words, the footer hints at the right | (in the tree, uncommitted) |
| T3 | S3, S4 | The main menu: the two-line rows with icons and subtitles, the group titles, the selected row's detail panel and its button; the 80x24 floor keeps the one-line rows | (in the tree, uncommitted — layout and vocabulary in the current palette) |
| T4 | S5 | The dashboard cards, the greeting with the user's name, the tagline and the call to action | (in the tree, uncommitted — the dashboard, the cards and the named greeting) |
| T5 | S6 | The setup table and its right panel | (in the tree, uncommitted — the table, its detail panel and two goldens) |
| T6 | S8 | The utilities screen | (in the tree, uncommitted — the card of two-line rows, the System Overview panel and two goldens) |
| T7 | S9 | The theme selector grid | (in the tree, uncommitted — the heading, the subtitle, the card grid, the floor list, the tip and the goldens) |
| T8 | S7 | The installing checklist, the log panel and the progress bar | (in the tree, uncommitted — the mockup's stack, the checklist beside the log card, the bar under both and the `[q] Cancel` footer) |
| T9 | S10 | The remaining pages | (in the tree, uncommitted — the pickers' card, the framed dead ends, the completion fact rows, the backup cards and the learn pages, the keymap tables and the LazyVim guide on the reference card; the doctor and the update center are follow-ups, see L16 and L19) |
| T10 | S11 | The floor/stability/contrast guards and the goldens | (with each task) |
| T11 | S2, S7, S10, S11 | The update center's live progress: the run's own bar and count, the command in flight and the entries it moves, the entries it did not move, and the end of the manager's output — as the wide frame's card and as the 80x24 floor's four rows | (in the tree, uncommitted — see L33) |

T3 now ships in the current palette so the layout and the vocabulary can be
reviewed before the palette changes; T1 recolours the same grammar.

## Log

L1. User: "Una vez tengas todo, tengo esta imagen, para un refactor bastante
grande la interfaz de los dotfiles. Puedes hacer algo así, y ajustar el resto de
pestanas que no salen en la iamagen, para seguir el mismo estilo estetica y
visual que estas pantallas, para dejarlo todo perfectamente
/tmp/pi-clipboard-871b9484-2534-4d60-931c-26fbf88db120.png."
L2. User: "continua con la remodelizacion de la interfaz".
L3. Decision: the current fix branch (#393) lands first; this front starts on
main. T3 is the first slice, in the current palette, with the existing IA (the
update and tools rows stay); the six-row Learn/Session hub consolidation follows
as T3b.
L4. User: "pues continua, no pares hasta que tengas todo solventado. si lo
necesitas, tienes el resto de sesiones herdr dotfiles para apoyarte por si
necesitas paralelizar algo. Si van a hacer algo, tienen que leer si o si la
imagen que hablamos. La paleta de colores no hace falta que la copies
exactamente, adaptala a los theme que tenemos, pero el diseño de la UI si me
gustaria que la copiases en la medida de lo posible".
L5. Decision: the design is copied as closely as the themes allow; the palette
adapts to the existing themes (no new violet theme). T3's first pass is
uncommitted on `feat/installer-redesign`: the main menu's two-line rows, icons,
subtitles and detail-panel button are in; `TestMainMenuPlanPanelLeavesRoomForTheCompanion`
is red (the wide/floor plan+companion arithmetic), and the T2 chrome is next.
Anyone working on this front must read the mockup image first.
L6. T2 + T3 landed together in the tree (uncommitted, still the current palette):

- **The two-line row is conditional on the frame's other tenants.** A row costs
  two rows, so `mainMenuTwoLine` takes the mockup's shape only where the rows the
  body does not need can still hold the active panel's summary *and* the
  creature at the terminal's rung. That is what `TestMainMenuPlanPanelLeavesRoomForTheCompanion`
  was red about: at 60x20 the two-line card spent the summary's row, so the plan
  stopped answering. At 80x24 (no update rows) the two-line card, the plan
  summary and the pet all fit; with an extra menu row the floor falls back to the
  one-line card. The guard logs `rows=one-line|two-line` so the shape is
  re-derivable, and the `impossible` records keep their arithmetic.
- **The greeting stays on the floor shape only.** The mockup's main menu has no
  greeting and no card title; the one-line card keeps both, because there the
  card is the compact list and the greeting is the screen's only prose. The three
  greeting guard tests now read the shape from the model instead of restating a
  screen name.
- **The status bar rides the footer rule, not the legend's row.** Sharing the
  legend's last row cost six screens a second footer row at 80 columns
  (install-review, wsl-migration, ssh-host-read-only, theme-picker/activity/
  refresh, network-diagnostics), and on the review screen that row is the
  documented "Way back" marker: a floor regression. The rule above the legend is
  a row with nothing else to say, so the state block leads it and the rule runs on
  -- zero rows, zero columns, and the keys keep their full measure and their
  right-aligned edge. The trainer composes its own frame with no footer rule, so
  there the block shares the legend's last row (`footerBar`), and its budget uses
  `footerBarRows`. State words are exactly the mockup's five.
- **The panel's button** is the two-column panel's last row; the one-column
  summary is one row and cannot spend it.
- Follow-ups this slice found and did not take: `deadEnd` still renders a two-line
  screen with no chrome (T9 should route the dead ends through the frame), and the
  status dot's tone per state (Ready/Done green, Installing brand, Preview accent,
  Failed error) is a role choice T1 may want to re-read against the new surfaces.

L7. T4 landed in the tree (uncommitted, still the current palette): the dashboard,
which is the welcome screen under the lockup. What was copied from the image
(panel 1, "Dashboard (inicio mejorado)"), in the mockup's order:

- **The lockup and its tagline.** The emblem and the wordmark stay the installer's
own art; the line under them is the mockup's `Your personal development
environment.`, replacing "Your terminal environment, configured in minutes."
- **The four machine cards.** `OS`, `Shell`, `Arch`, `Packages`, each with its Nerd
  Font icon and the value the model holds: `OSName`, `UserShell`, `archName(info)`
  and the same `packageManager()` the dependency step dispatches through. One card
  shape, three rows: the icon and label ride the top border (the frame's own card
  grammar, `panelBlock`'s title) and the value sits inside, because the mockup's
  fourth row is the one that does not fit the 80x24 floor with the panel summary
  still showing. The cards are equal width and exactly as wide as the body; below
  the width where a card can hold its own value they collapse to the four facts in
  one wrapped row (60 columns), which loses no fact.
- **The greeting by name.** `Good morning, <user>!` from the mockup, read from the
  model's own clock (`greetingFor`) and from the login name the installer read once
  at startup (`currentUserName`: `USER`, `LOGNAME`, `USERNAME`, then
  `user.Current()`), carried on the model as `UserName` so no render probes. A host
  that cannot name its user gets `Good morning!`; the mockup's waving-hand emoji is
  dropped (no emoji on any screen, S3's glyph rule).
- **The environment sentence** stays the installer's own fact sentence, in the
  mockup's slot: `Running on ... under WSL, with Homebrew already installed
  (dev build)`.
- **The call to action** `Let's make this machine yours.` in the mockup's italic.
- **The pet** keeps the frame's existing placement: the rows the body did not need,
  at the left, under the content -- which is where the mockup puts it. It appears
  where the frame allows (the body leaves whole rows) and is dropped where it does
  not; nothing is taken from a body to draw it.
- **The vertical anchor.** The welcome screen used to centre its body and was the
  one exception to `anchorBodyToTheRule`. The dashboard's height changes with the
  machine it describes, so a centred body moved the lockup between two hosts and
  spent the rows above that the creature below could not use. Every screen now
  anchors to the rule, which is what puts the lockup under the header and the pet
  at the bottom, as the mockup draws them.

The floor and the guards. At 80x24 the compact lockup is kept, the body spends 18
of its 19 rows with the golden host and the frame's panel summary keeps its row;
the lockup's blank row is the first row given back when the frame is tighter
(60x20 spends all 15 and keeps its footer). `TestEveryScreenFitsEveryTerminalSize`
was red at 140x44 and 60x20 until the card width arithmetic (`need` counted the
title's spaces wrong) and that blank row were fixed. `goldenSystemInfo` now names
`Arch: "x86_64"`, because detection always fills it and a golden host without it
pinned a three-card dashboard; `isolateGoldenTest` pins `goldenUserName` for the
same reason. New guards: `TestDashboardCardsAreTheMachinesOwnFacts`,
`TestDashboardCardValuesAreNotInvented`,
`TestDashboardGreetingUsesTheModelClockAndTheUsersName`,
`TestDashboardGreetsByTheLoginNameTheInstallerCanRead`,
`TestDashboardFitsTheFloorAndKeepsItsParts`,
`TestDashboardCardsCollapseToFactsWhenTheyCannotFit`,
`TestDashboardCardBandIsExactlyTheBodysWidth`. New golden:
`TestDashboardWideGolden` (160x40, the mockup's layout); `TestWelcomeScreenGolden`
and `TestWelcomeLivePanelGolden` were regenerated.

Follow-ups this slice found and did not take: the mockup's pet sits beside the
greeting (ours sits under the call to action, because the creature is the frame's
and not the body's); the palette is still the current one (T1); the greeting's
name is the login name only, with no display-name lookup (`GECOS`).

L7. User's redesign guidance (verbatim directives, latest): "No reinicies el
trabajo ni replantees todo el diseño." "Cada pantalla debe tener una jerarquía
visual clara: título, contenido principal, información secundaria y acciones."
"Evita llenar la interfaz de bordes, cajas e iconos innecesarios." "Conserva la
identidad retro, el pixel art y la mascota." "Utiliza una paleta coherente, con
el violeta como color principal." "Los colores deben transmitir significado:
selección, éxito, advertencia, error e información." "No dependas de dimensiones
fijas... Evita desbordamientos horizontales y textos cortados incorrectamente."
"Reutiliza los componentes existentes... Centraliza estilos, colores, tamaños y
espaciados." "Mantén separada la lógica de instalación de la presentación."
"Soporta las teclas ya existentes (j/k, flechas, Enter, Esc, Tab, Space)."
"Muestra solamente los atajos relevantes para la pantalla actual." "Mantén el
estado de selección al volver de una pantalla secundaria." "Nunca bloquees la
interfaz esperando una operación larga." "No muestres porcentajes de progreso
ficticios." "Conserva y respeta las políticas existentes de backup, restauración
y sobrescritura." "Evita animaciones constantes." "Añade pruebas donde aporten
valor, especialmente para navegación, cambios de estado y cálculos de layout."
"Revisa primero los cambios que ya están en curso... Continúa la implementación
por pantallas... Valida cada pantalla... Evita cambios ajenos al rediseño."

L8. Decision: the dashboard/menu/chrome draw through the theme roles, so a
palette change reaches them without reworking their layout. Every remaining task
follows L7: the shared frame/header/status/panel/list grammar, no invented facts
or percentages, the existing install policies, the 80x24 floor and the resize
matrix, and no unrelated changes.
L9. User: "Sobre los colores, manten la logica que tenemos de themes, que todo el
installer (y todo el sistema) se ajuste al theme aplicado mediante el instalador,
y recuerda ir visualizando cada una de las pantallas con VHS". Decision: no new
theme and no hardcoded palette; the applied theme (the installer's own switch)
drives the installer and the system, exactly as before. The mockup's violet is
illustration only. Every slice is captured with the existing VHS harness
(`make screens`, 200x55 and 80x24) and the captures are reviewed before the slice
is called done.

L10. User: "planificalo para que en cuanto vuelva el subagente, iteres de forma
autonoma, sin lanzar ningun push ni nada (hasta que no vuelva yo, para que el chat
no se quede bloqueado por falta de aprobacion del comando)". Decision: the
autonomous loop between worker completions is (1) review the diff against the
image and the task, (2) run the focused tests, the golden update and the full
suite, (3) run `make screens` and review the 200x55 and 80x24 captures, (4)
commit locally in a work unit, (5) launch the next slice (T5, T6, T7, T8, T9 and
T1 last), repeating. While the user is away there is no `git push`, no PR update,
no merge and no command that needs the human's approval. A slice blocked by a
design decision, a failing guard the design cannot resolve, or a missing tool
stops the loop with a concrete blocker report; it is never pushed and never
worked around silently.

L11. User approved the test-loop change: during a slice run only the focused
tests, the affected goldens and the VHS captures; the full suite and
`make preflight` run once before the PR (and in CI). The truecolor-golden removal
and the matrix reduction are a separate, later decision taken after the redesign
settles, so the safety net is not thinned while the layout is still moving.


L12. T5 landed in the tree (uncommitted): the setup table, its detail panel and
two goldens. What was copied from the image (panel 3, "Setup (configuración
interactiva)"), in the mockup's order:

- **The table's own title and subtitle.** `Configure your environment` centred
  over the table with `Each option is configured on this screen. Press Space to
  toggle.` under it, both from the mockup. The subtitle is the screen's
  description in the model (`GetScreenDescription`), so there is one copy of the
  sentence, and the screen's title is the same word pair the body draws
  (`setupTableTitle`) instead of the step number said twice.
- **The row.** The gutter holds the row's Nerd Font icon -- or, on the row under
  the cursor, the cursor's own marker in the icon's place, as the mockup draws it,
  so the name never moves -- then the name at the left, the value at the right and
  the chevron in the last column. The values are the rows' own
  (`setupRowValue`), unchanged: `[x] install`, `[ ] skip`, the chosen option,
  `[x] back up`, `[x] always installed`, `[ ] leave it out`.
- **The chevron is the rows Enter opens**, and only those: a plugin set has no
  screen of its own (Space toggles it where it stands), so the two set rows carry
  none -- which is exactly how the mockup draws its Oh My Zsh and Multiplexer
  rows. The glyph is the Nerd Font's angle-right (U+F105) rather than the
  dingbat the spec named (U+276F): the dingbat sits in the block the emoji-box
  guard covers, and that guard is older than this slice.
- **The selected row** is one bar across the table's width with the accent
  marker, the name bold and the value on the fill -- the same pair of tones
  `rowBar` has always used (`RowBarMarkedStyle`/`RowBarStyle`), so a palette
  change reaches it without touching the layout.
- **The table itself.** Where the frame can hold it: the mockup's rounded box, a
  rule between every pair of rows, one blank column inside each border, and the
  values at a shared column three fifths of the way across. Where it cannot: the
  same rows without the box, with each row ending its value at its own right edge.
  The shape is the first candidate -- box+panel, box+summary, list+panel,
  list+summary, list alone -- whose lines fit the frame, so the richest shape the
  frame can hold is the one drawn and a shape that would hide a row is never
  chosen over one that would not.
- **A row that cannot share its line** puts the value on the row's second line,
  under its name. At 80x24 the fish's plugin set is the one that needs it: its
  name is the longest (53) and its value the widest (20), and the sum does not fit
  -- the previous renderer cut three columns off that row at the floor, silently,
  which the new no-cut guard would fail on. At 60x20 the two-line shape carries
  the rows the window shows.
- **The band under the table** is a `panel`: the selected row's name as its chip,
  the rule under it, and the description as its facts, wrapped at 64 columns and
  placed in the right half where the mockup puts it. Where the panel body would
  cost the list a row, the machinery's own one-line summary is drawn instead
  (`Platform  Select your operating system.`), and where the frame has room for
  neither, nothing. The reservation is the widest row's, not the selected row's,
  so the band cannot change height as the cursor moves. The panel states no value:
  a guard fails if a description ever repeats the value of the row beside it.
- **The pet** is still the frame's, and it lands under the band -- the mockup
  draws it beside the description, which would mean the body placing a sprite the
  frame owns. At 160x40 both fit; at 80x24 the list and the summary take the rows
  and the creature is dropped, exactly as the frame drops it everywhere else.

The floor and the guards. At 80x24 the shape is the plain list with the summary:
11 rows of the worst case (fish's, the multiplexer's and the editor's sets and a
backup row) spend 11 of the 13 lines the frame leaves, the summary keeps its row,
and the creature takes the three that are left. At 160x40 the box, its row rules,
the panel body and the pet all fit. New guards: `TestSetupTableDrawsEveryRowWhole
AtEveryMeasuredSize` (no cut marker at any of the twelve measured terminals, the
cursor's row always drawn, the hidden rows counted),
`TestSetupTableTakesTheMockupsShapeWhereItFits`,
`TestSetupRowsCarryTheChevronOnlyWhereEnterOpensSomething`,
`TestSetupRowGlyphsAreOneCell`, `TestSetupEveryRowCarriesItsDescriptionAndNoValue`,
`TestSetupScreenDoesNotMoveWhenTheCursorMoves` and
`TestSetupRowWindowKeepsTheCursorRowAndCountsWhatItHides`. New goldens:
`TestSetupScreenGolden` (80x24) and `TestSetupWideGolden` (160x40).

Follow-ups this slice found and did not take: the VHS tapes capture the welcome,
the main menu and the utilities, so the setup screen has no capture (its shape is
pinned by the two goldens instead; adding a setup step to the tapes is a harness
change outside this slice); the footer still says `choose` where the mockup says
`Next`, which is the key grammar's call and not this screen's; and the pet sits
under the band rather than beside it.

L13. T6 landed in the tree (uncommitted): the utilities section, its System
Overview panel and two goldens. What was copied from the image (panel 5,
"Utilidades (diagnósticos mejorados)"), in the mockup's order:

- **The card of two-line rows.** The job's Nerd Font icon in the gutter, its name
  on the first line and its one-line description under it, the row under the
  cursor filled across the card with the cursor's own marker in the icon's place
  (`RowBarMarkerStyle`/`RowBarStyle`, the same pair the setup table uses), and the
  rounded box, one blank column inside each border, around the whole list. The
  subtitles are the mockup's: `Measure startup time`, `Check latest versions`,
  `Manage SSH keys`, `Check connectivity`, `Find an open port`, `Remove temporary
  files`, `Check common issues`. The rows this installer has that the mockup does
  not show (the theme switch, the WSL resources, the terminal capabilities, the
  verification) carry a description in the same voice. The names are the
  mockup's too: `Shell Startup`, `Installed Tools`, `SSH Configuration`,
  `Network Diagnostics`, `Find Available Port`, `Clean Dev Caches`, `Diagnostics`,
  `Diagnose WSL`, `Back`.
- **The shape yields, the rows never do.** The two-line row is drawn only where
  every name and every description fits the card's content whole; a description
  cut mid-word would be worse than a list without one, which is why the width
  decides and not the height. At the 80x24 floor the same rows are drawn on one
  line in the same card, so all eleven jobs are on screen at once; a list too tall
  for any shape is windowed around the cursor and says `↑ N more above` /
  `↓ N more below`, and the header's vital names the slice (`Showing 3-9 of 11`).
- **The right panel, `System Overview`.** The machine's own facts with the panel's
  fixed label measure -- `OS`, `Host`, `Arch`, `Shell`, `Packages`, `Home` -- then
  `Resources` with a meter and its number for `CPU`, `Memory` and `Disk`, then the
  doctor's verdict `✓ Everything looks good!` in the success role. `Resources` is
  the mockup's card: its heading on the top border and the meters inside, one card
  rather than a rounded box per meter, which is two rows instead of six and is
  what keeps the block and the verdict on screen together. A resource appears only
  where its own flag says the sample reported it, and the whole block is absent
  while the sampler is gated off: a stale reading drawn as a current one is the
  lie the gate exists to prevent.
- **The floor keeps the panel's summary.** Below the two-column floor there is no
  column for the panel, so its summary rides the card's top border
  (`╭─ System Overview · Linux · x86_64 · zsh · apt-get ─╮`) exactly as the frame
  collapses a panel everywhere else, and the body is centred in the composition.
- **The actions and their key grammar are untouched.** Every row is an entry of
  `GetCurrentOptions`, so the cursor index, the handler's match and the drawn row
  are one list; the reasons a job is missing stay above the card, the theme
  switch's result stays under it, and the footer names the three theme keys only
  while their rows are offered.

Two things the mockup draws are deliberately not row for row:

- **Its first row, `System Overview / Check environment status`.** That row exists
  in the mockup because the mockup's right column is the *selected row's* detail;
  here the panel is contextual -- it states the same machine whatever the cursor
  is on -- so a row whose only job would be to show it would be a row no key can
  act on. The overview is the panel's title (and the floor's card summary)
  instead, and `TestUtilitiesCardDrawsOnlyTheRowsTheHandlerMatches` fails if the
  card ever grows a row the handler does not match.
- **Its `Kernel` and `Uptime` rows.** Nothing in this installer reads either
  (detection reads the kernel's own string once to recognise WSL and keeps none of
  it; nothing reads `/proc/uptime`), and a row reading `unknown` would be the panel
  guessing rather than reporting. The panel's own guard fails if either ever
  appears without a reader behind it.

The floor and the guards. At 80x24 the body spends two rows on the reasons, the
card's eleven rows and its two borders, and the summary keeps its row on the
border; the golden is unchanged by this slice's polish, which is the point. At
160x50 the two-line card, the whole panel, the Resources card and the verdict all
fit. New guards: `TestUtilitiesRowsDrawTheMockupsNamesAndDescriptions`,
`TestUtilitiesCardDrawsOnlyTheRowsTheHandlerMatches`,
`TestUtilitiesRowGlyphsAreOneCell`,
`TestUtilitiesTwoLineRowsAreTheMockupsAndCostWhatTheyMeasure`,
`TestUtilitiesEveryJobStaysReachable` (every row, every measured terminal),
`TestUtilitiesWindowNamesWhatItHid`,
`TestUtilitiesOverviewStatesTheFactsTheModelHolds`,
`TestUtilitiesResourcesFollowTheSamples` (including the Resources card),
`TestUtilitiesOverviewSaysEverythingLooksGoodOnlyWhenTheDiagnosisSaysSo` and
`TestUtilitiesFloorKeepsTheCardAndTheFacts`. New golden:
`TestUtilitiesWideGolden` (160x50, the mockup's layout);
`TestUtilitiesGolden` (80x24) was regenerated for the mockup's row names.

Follow-ups this slice found and did not take: the rows are drawn in the option
list's own order (the model's: the theme and WSL rows first, the doctor last)
rather than the mockup's, because reordering the list would move every cursor and
handler index -- that is a key-grammar call and belongs to whoever takes it; the
mockup's monitor row and its per-meter boxes are the two deviations above; the pet
sits under the card rather than beside the panel, as the frame places it
everywhere; `sectionPaneTitle`'s `Notes` and the utilities entry of
`usesSectionBands` are now unreachable, because the section composes its own body
-- removing them is a `section_bands.go` cleanup outside this slice; and the VHS
tapes still capture the utilities screen with the old row names until `make
screens` runs.

L14. T7 landed in the tree (uncommitted): the theme selector, its card grid, its
floor list and four goldens. What was copied from the image (panel 6, "Theme
Selector (extra)"), in the mockup's order:

- **The heading and the subtitle.** `Choose a theme for your installer` centred
  in the brand tone and `This will change the colors and style of the interface.`
  under it in the muted one -- the mockup's words, verbatim, and the words the
  screen now says instead of the long account of what applying writes. That
  account is not lost where it matters: when the list cannot be read, the screen's
  own reason takes the subtitle's place (a failure the user cannot read is the one
  thing this screen may not drop), and the undo row, the cards' exclusion notes
  and the tip carry what the paragraph used to explain.
- **The grid of cards.** Two cards to a row at the wide frame, each a rounded box
  holding a miniature of a screen painted in that theme's own colours -- its base
  as the fill, its accent as the title chip, its selection as the filled row, its
  text as the body line and its palette as a strip of swatches -- and under each
  box the mockup's mark: `●` on the card the cursor is on, `○` on the others, with
  the theme's name beside it and, where a theme leaves tools out, the same
  exclusion note the option row carries. The miniature is drawn from the
  definition the switch would write, so it cannot show a colour the apply would
  not; the palette is not a second palette.
- **The tip.** `Tip — You can change the theme later from the utilities menu.`,
  the mockup's lightbulb glyph in the warning tone, under the grid.
- **The status.** `Preview mode`, which the frame's status bar already carried:
  this slice adds no state and changes no word.
- **The floor's single list.** At 80x24 the same themes are one row each: the
  mark, the name, the exclusion note and the palette strip the card carries, with
  the theme's row filled in the row-bar tone the whole frame uses and the strip
  after it in the theme's own colours. Seven themes, the tip, the refresh row and
  the way back all fit at 80x24; a longer list is windowed around the cursor and
  the header says `Showing N-M of T`, and below 60 columns the strip drops whole
  swatches rather than splitting one.

The shapes are a ladder, not a switch: two columns of cards, then one column of
cards, then the list, each tried with the tallest miniature it can carry (four
rows, then three) and with the tip if there are rows for it. The richest shape the
frame can hold is the one drawn, and a grid that would hide a theme behind a note
is never chosen over the list that would not. The action block (the rule, the
refresh row, the undo row and the way back) keeps its rows and its letters: the
refresh and the undo still run on their own keys, and every row the handler
matches is still reachable by the cursor and drawn.

The floor and the guards. At 80x24 the body spends the heading, the subtitle, the
seven rows, the tip, the reserved action block and the reserved notice band, with
the palette board absent (one column) and the cursor's mark on the first card. At
160x50 the two columns of cards fit beside the palette board, which the picker has
always offered; the mockup's theme selector has no board, so the grid is drawn in
the left column and the board keeps the right one. A press keeps every theme on
screen -- the result of a press takes the rows the tip and the notice were holding,
never one of the list's -- so the pending line and the result land in the same
rows and the cursor does not move. The prose is the one thing that is cut when the
frame cannot hold it, and it is cut with the frame's own marker.

New guards: `TestThemePickerDrawsTheMockupsGridWhereTheFrameHoldsIt` (two boxes to
the first grid row and the tip at 160x50; no box at all at 80x24; every theme
named whole at both), `TestThemePickerSaysTheReasonWhenThereIsNothingToChoose`
(the empty state states why at every measured terminal and fits it),
`TestThemeCardMiniaturePaintsTheDefinitionsOwnPalette` (every fill the miniature
paints is one of the definition's colours, within lipgloss's one-step rounding,
and the base, the accent and the selection are painted). Two existing guards were
re-pointed at the new grammar rather than weakened: the two that walked the list
now assert the row under the cursor carries the mark and names the theme the
option selects (`themePickerMarkedRow`, `themePickerDrawnLabel`), and the one that
watches a press no longer requires the option text to be drawn on the card.
New golden: `TestThemePickerWideGolden` (160x50, the mockup's grid);
`TestThemePickerGolden`, `TestThemePickerActivityGolden`,
`TestThemePickerResultGolden` (80x24) and `TestAppliedThemeGolden` were
regenerated.

Follow-ups this slice found and did not take: the frame's header chip still says
`Utilities` where the mockup's says `Theme Selector` (the chip is the chrome's, and
every utility screen shares it -- changing it is a chrome decision, not this
screen's); the refresh review and its result still render through the section
renderer, so the screen's heading changes to `Change the dotfiles theme` in those
two states; the palette strip sits at the right edge of the floor's row rather than
beside the name, which is a reading-width call; the cursor still walks the grid in
reading order one row at a time (a two-column grid could move by two with `j`/`k`,
but that is the key grammar's decision and belongs with it); and the VHS tapes
still capture the picker with the old list until `make screens` runs.

L15. T8 landed in the tree (uncommitted): the installing screen, its checklist, its
log card, its bar and the footer's own way out. What was copied from the image
(panel 4, "Instalación (progreso detallado)"), in the mockup's order:

- **The stack over the two columns.** `Running installation...` centred in the
  brand tone, and under it `Step 3 of 16 - Install Dependencies` -- the mockup's
  dash, not the `·` the wizard's counters use -- then the run's own clock
  (`Elapsed 42s · ~3m 46s left`, or the sentence saying it is still estimating).
  The three rows are a fixed count: a run whose plan is not built yet keeps its
  step row blank, so the columns under it never move. The header chip is the
  mockup's word (`Installing`, where it said `Installing dotfiles`) and the frame's
  status rule still carries `Installing…`.
- **The checklist.** One row per step: the state glyph, the name, and the step's
  own progress as the trailing meter the row has always used. The step the run is
  on is one filled bar across the column with its glyph riding it, which is how the
  mockup draws the running row; every other row is plain. The five states are the
  existing five (`✓` done, `●` running, `✗` failed, `⊘` skipped, `○` pending) and
  the glyph, not the tone, is what carries them. The checklist is drawn bare -- the
  mockup draws no box around it -- and windowed around the running step, so the
  header's `3/16` is the only place the count is said.
- **The log card.** The mockup's rounded box at the right, every line led by its
  `> ` prompt, the freshest lines kept and one dim row naming the earlier ones the
  card could not show (the existing `logTailCount` arithmetic). A run that has
  logged nothing yet says it is waiting instead of drawing an empty box, so the
  card the frame reserved is never mistaken for one that failed to draw.
- **The bar.** Under both columns, filling the composition, with the percentage at
  the frame's right edge as the mockup puts it. It is `installProgress`'s fraction
  and nothing else: done and skipped count fully, the running step counts what it
  reported, pending counts zero.
- **The footer.** `● Installing… ───...` on the rule and `[d] details • [q] Cancel`
  on the legend, which is the mockup's one way out beside the toggle that
  collapses the log card.

Two things the mockup draws are deliberately not row for row:

- **Its per-step times (`2.3s`, `18.7s`).** Nothing in this installer records a
  per-step duration: the run holds one start timestamp and the latest tick, and a
  step holds its status and the fraction it reported. A row that showed `2.3s`
  would be the screen inventing a clock, so the trailing column is the meter the
  step really has -- and the run's own elapsed time and estimate, which the model
  does hold, stay on the screen as the third row of the stack. A guard fails if a
  checklist row ever states a duration.
- **Its `[q] Cancel` needed a key.** The footer's promise is honest only if q acts,
  so q now stops the run through the path ctrl+c has always taken: the program
  quits, the running step stops where it is, and no record is written because
  nothing failed. That is one flow with two keys, not a second cancel beside it;
  the install flow, the operations and the backup/restore/overwrite policies are
  untouched, and `TestAnAbortedRunLeavesNoRecord` still holds for the new key's
  path. The guard that pinned "q does not quit during installation" now reads
  `ScreenOSSelect` instead, because the installing footer is no longer the case it
  describes.

The floor and the stability. At 80x24 the body spends the three stack rows, a
fifteen-row band (the checklist's fifteen rows beside a thirteen-row card) and the
bar, with the machine's pulse in the rows above the bar when the sampler has
landed. At 60x20 the band is eleven rows and the log card is still drawn: its
columns are 30 and 24, and the checklist drops its trailing meter entirely rather
than cutting a step name -- a name is a fact, the meter is a decoration, and the
run's bar still carries the progress. The band's height comes from the frame and
never from the log's length, so the bar cannot move while the run writes, and
`TestInstallingLayoutDoesNotMoveAsStepsAdvance` pins it across a step boundary.

New guards: `TestInstallingScreenCopiesTheMocksShape` (the stack, the band, the
card's borders, the bar's percentage and the footer, at 80x24 and 160x50),
`TestInstallingChecklistStatesEveryStepTheRunKnows` (all five glyphs and the meter
each state earns), `TestInstallingDoesNotInventStepTimes` (no row states a
duration; the run's own clock is shown), `TestInstallingListRowsSayWhatTheyMeasure`,
`TestInstallingFloorKeepsTheCurrentStepAndTheLog`,
`TestInstallingColumnsSplitTheFrame` (the two columns fill the composition with one
gap at every measured terminal), `TestInstallingLayoutDoesNotMoveAsStepsAdvance`,
`TestInstallingBarIsTheRunsOwnProgress` (including the no-plan run reading 100%),
`TestInstallingFooterStopsTheRunOnQ` (and the same stop from ctrl+c, and while the
WSL offer waits), `TestInstallingLogKeepsTheMockupsPrompt`,
`TestInstallingChecklistNeverCutsAStepName` (the meter ladder),
`TestInstallingEveryMeasuredFrameDrawsTheWholePlan`, `TestInstallingHeadAlwaysCostsTheSameRows`
and `TestInstallingStepCounterStaysInsideThePlan`. New goldens:
`TestInstallingScreenGolden` (80x24, the two columns) and `TestInstallingWideGolden`
(160x50, the mockup's layout); `TestInstallingLiveGolden` and
`TestInstallingTruecolorGolden` were regenerated.

The run now opens with the log card on (`beginInstallFromReview`), because the
mockup draws it and the lines are what the step being watched is writing; `d` still
collapses it, and the checklist then takes the whole composition while the bar
keeps its row. Nothing about the details key changed except its default.

Follow-ups this slice found and did not take: the mockup's per-step times would
need a per-step clock on the model, which is a run-state change and not a render
one; `styles.BoxStyle` lost its last caller (the old installing log box) and is now
unused -- removing an exported theme style belongs with T1's palette pass; the VHS
tapes do not capture the installing screen at all, so its shape is pinned by the
goldens rather than by a `make screens` frame; and the checklist's window still
says nothing about the rows it hides, where the setup table and the utilities card
both name theirs (`↑ N more above` / `↓ N more below`) -- the header's `3/16` is
the count, and adding the notes would spend two of the band's rows.

L16. T9 landed in the tree (uncommitted): the pages the mockup does not draw,
brought onto the grammar the mockup established, one screen family at a time.
Five things were unified, each with a guard and its goldens:

- **The wizard's own questions draw the menu's card.** The six pickers and the
  terminal warning are menus -- a title, a description and a list of answers --
  and they drew a bare list where every hub one screen later drew the frame's
  card. `renderMenu` and `renderSelection` now share one body builder
  (`menuCardBody`), so a question and a hub cannot be two shapes: the page's title
  in the brand tone, the model's own description under it in the muted one, the
  answers in the card with a divider where `menuOptions` has a separator, and the
  cursor's row as the bar. The answers, their order, the keys, Choosing's Enter,
  the panels and the wizard's step counter are untouched. The description is now
  capped by the frame instead of by a constant: `menuCardBody` builds the head,
  measures what the body is over its budget and takes the rows back from the
  description (with the frame's cut marker), because the answers are facts and the
  prose is not -- which is what fixed `terminal-select-wsl` and `ghostty-warning`
  drawing one row past the frame once the card's two borders arrived. The body
  builder also wraps a description that holds its own line breaks:
  `wrapText` measured the `\n` as a column and the renderer drew it as a row, so
  every `description + "\n" + sentence` note had been spending a row nobody
  counted.
- **The dead ends wear the frame.** `deadEnd` drew two bare lines -- no header
  chip, no state, no legend -- which was the one screen shape in the program that
  was not the frame, and the follow-up L6 left. `deadEndScreen` draws the missing
  fact in the error tone and the way out under it, and the legend carries
  `[Esc] back`: the key is named once, so the sentence keeps only where it goes
  (`deadEndWayOut` drops the `Press [Esc] to ` prefix and capitalises; a guard
  pins every sentence the screen says). The trainer's frame is composed row by row
  (`trainerDeadEnd`), so its two unreachable dead ends get the trainer's chrome and
  not the installer's.
- **The completion summary draws the frame's fact rows.** `OS: macOS` was the one
  fact block in the program that spelled each row as a sentence; the summary is
  now `panelFact`'s label column and value, the same pair the utilities panel and
  the review draw, so a reader compares values down a column. `fontSummaryLine`
  became `fontSummaryValue` (the label moved into the label column) and its guard
  reads the value.
- **The backup screens draw their answers in the card.** The three screens put the
  rows a person presses in the frame's card -- the restore history with its `←
  Back` row behind the shared divider, the confirmation's contents in a card titled
  on its border, the answers in a card under it -- and the fixed-row reservations
  and two new guards follow (`backupConfirmFixed` 6→8, `restoreBackupBodyFixed`
  5→7, `restoreConfirmBodyFixed` 7→10, each counting the borders it now spends).
  Copy, order, defaults and the backup/restore/overwrite policies are unchanged.
- **The learn tables draw their entries whole, in two lines.** The three
  read-only tables held three fixed columns at every width, so at the 80-column
  floor the Kind column was three letters and a cut marker (`File li…`) while
  every description lost its tail -- both facts unreadable at once, beside 19
  empty columns of a quarter-width name column. A learn page now draws each entry
  as the frame's two-line row: the name at the table's shared name column, its
  kind beside it in the muted tone, and the sentence under both, wrapped to the
  page and never cut. The shape is the table's own (`referenceTable.TwoLine`), so
  the keymap tables keep their three columns, where the columns are the page. The
  window moved with it: `referenceWindow`, `referenceEntryCost` and
  `referenceMaxScroll` are one set of helpers the renderer and the key handler both
  read, so a page of two-row entries is scrolled by the rows it draws and the last
  entry is reachable (`handleReferenceTableKeys` now takes the table instead of a
  count). The vitals read `Showing 1-7 of 16` where a page now shows fewer entries
  than it holds. The pros and cons of a tool page wrap the same way
  (`wrappedListRows`): an entry is drawn whole or counted in the `… and N more`
  note, and the rows the pros did not need go to the cons instead of being left
  blank under a note.

The floor and the guards. At 80x24 every page still fits the frame at all twelve
measured terminals (`TestEveryScreenFitsEveryTerminalSize`,
`TestInstallerScreensFitTheFrame`), and the wizard's step counter, the restore
history's window and the learn pages' scroll range all keep their last row
reachable. New guards: `TestPickerQuestionsDrawTheFramesCard`,
`TestPickerQuestionsKeepTheirKeysAndTheirWays`, `TestDeadEndsWearTheFrame`,
`TestDeadEndWayOutKeepsTheDestination`,
`TestCompletionSummaryDrawsTheFramesFactRows`,
`TestBackupAndRestoreDrawTheirAnswersInTheCard`,
`TestLearnTablesDrawEveryEntryWhole`, `TestLearnTablesWindowInTheRowsTheyDraw`
and `TestToolInfoWrapsItsReasonsInsteadOfCuttingThem`. Two existing guards were
re-pointed rather than weakened: `TestFontSummaryValueNamesWhatTheRunFound` (the
value without its label prefix) and the dry-run completion's multiplexer check
(the label and its value instead of `Multiplexer:`). Five goldens were
regenerated: `TestOSSelectGolden`, `TestBackupScreenGolden`,
`TestCompleteScreenGolden`, `TestCompleteTruecolorGolden` and
`TestCompleteCelebrationGolden`. A new opt-in capture
(`TestCaptureEveryRedesignedPage`, `DOTFILES_CAPTURE_DIR`, the existing harness'
convention) writes the pages this slice touched at 80x24 and 160x50 so the next
slice can read them.

Follow-ups this slice found and did not take:

- **The doctor and the update center** draw through `renderThemeScreen` and
  `section_bands.go`, the shared section renderer that the ports, the network
  diagnosis, the SSH screens, the WSL screens, the shell audit, the disk cleaner
  and the install verification also draw through. Their bodies are already the
  frame's title/detail/list/tail grammar, but their rows are one-line bars and
  their actions are rows inside the list rather than in the card. Changing that is
  one decision for ten screens, not a page's: it belongs to whoever owns the
  section renderer, and it is why this slice left those two where they were.
- **The wizard's pickers still draw their answers on one line**, where the main
  menu's card draws two. The subtitles exist for four of the seven questions (the
  terminal, the shell, the multiplexer and the editor have a one-line description
  in the tool tables this installer already ships) and not for the other three
  (the operating system, the font and the warning), so a two-line picker would be
  a row shape that appears for some questions and not others. Tying the pickers to
  those tables and deciding what the other three rows say is a copy decision.
- The completion screen has no body title of its own -- its header chip says
  `Installation complete` and the body opens on the summary card -- because the
  mockup's title line would repeat the chip or invent a sentence.
- `TestCaptureEveryRedesignedPage` lives in `zz_dump_test.go`, a name from the
  inspection harness the slice used; it wants renaming to `capture_redesign_test.go`
  when the slice is committed. (Done in the tree: the file is
  `capture_redesign_test.go`.)

L17. T1 landed in the tree (uncommitted): the theme pass. It changed no layout and
no colour value; it made the palette the only source of colour on the surfaces the
redesign added, proved it with guards, and removed what the redesign orphaned.

**The audit is measured, not read.** Every surface is rendered on a true-colour
terminal under the default chrome and under an applied theme, and every colour the
frame emits is checked against the palette it is wearing (`38;2;r;g;b` /
`48;2;r;g;b` read off the escapes, both members of every adaptive pair accepted).
The selector is the one surface allowed the definitions' own colours, because its
cards, its palette board and the preview swatches are pictures of other themes;
the exemption is written into the guard rather than assumed.

| surface | roles it draws through | guard |
|---------|------------------------|-------|
| header bar (chip and rule), status bar (dot, state word, key hints) | Brand, Success for the crown, OnFill on the chip, Rule, AccentKey/HelpVerb for the keys, Ink for the word | the role sweep on all 27 surfaces; the existing status-bar guards |
| two-line rows, group titles, the main menu's panel and its button | RowBar/RowBarMarker, Ink, Muted, Brand, Button (OnFill on BrandSoft) | the role sweep; the filled-style floor |
| the setup table and its band | PanelBorder, AccentKey for the icon, Ink, RowBar/RowBarMarker, Brand | the role sweep; the filled-style floor |
| the utilities card, the System Overview panel, the Resources card, the verdict | PanelBorder, AccentKey, Ink, Muted, Meter/MeterFilled, Success for the verdict | the role sweep |
| the theme grid, the card label, the floor's list and its strip | Brand, Muted, Ink, RowBar/RowBarMarker, Warning for the tip; the definitions' palettes for the pictures | the role sweep (with the definitions allowed); the miniature guard |
| the installing stack, checklist, log card and bar | Brand, Success, Error, Warning, Muted, RowBar, MeterOnBar, ProgressBarFilled/Empty | the role sweep; the filled-style floor |
| the completion, backup, restore and dead-end cards | Success/Error, PanelBorder, Ink, Muted, Brand, AccentKey | the role sweep |
| the remaining pages (pickers, learn tables, keymaps, doctor, update center) | Brand, Muted, Ink, PanelBorder, Success/Warning/Error, Code | the role sweep |

**Two things were actually wrong.**

- **The emblem's ramp was built once, at package init.** `var emblemRampStyles
  = []lipgloss.Style{...Primary, Secondary, Accent}` was initialised before
  `init()` ran `applyUIColors`, so all three stops held the zero value and the
  dashboard's lockup had no ramp in any theme -- not the default one either. It is
  `EmblemRampStyles` now, rebuilt with everything else in `applyUIColors`, and
  `renderEmblem` draws the lockup plain rather than indexing a stop that is not
  there. This is the case the builder guard was written for: a style that is not
  rebuilt cannot paint the palette it was given.
- **The section screens built their body title a second time.** While a preview was
  active they painted it from `def.Palette["blue"]` directly, where the preview
  had already repainted the brand role to that same blue: two paths to one colour,
  and the one path that could disagree with the chrome around it (a definition that
  cannot be previewed painted a blue nothing else wore). The title is `BrandStyle`
  and nothing else.

**What was removed.** `BoxStyle` (the old installing log box was its last caller,
T8) and `HelpStyle` (deadEnd's one-line way out; T9 routed the dead ends through
the frame, which left its own doc comment saying it was still used). Two styles are
unreferenced on this branch *and on main* -- `TitleStyle` and `DangerStyle` -- and
they are deliberately not this slice's: nothing the redesign dropped stopped using
them, so removing them is a separate cleanup rather than a piece of the redesign.

**The guards** (new, in `theme_audit_test.go`):

- `TestEveryRedesignedSurfacePaintsOnlyTheThemesRoles` -- the no-literal rule, 27
  surfaces at 80x24 and 160x50, under the default chrome and under an applied theme.
- `TestTheThemeSwitchRecoloursEveryRedesignedSurface` -- the switch reaches every
  surface (the theme's blue appears, the plain frame has none of it), and the paint
  is a render: clearing the record draws the bytes the model drew before, and the
  package palette is the one `View` started with.
- `TestApplyUIColorsRebuildsEveryStyleTheSurfacesDrawWith` -- with a palette whose
  every field is a distinct marker colour, every style var renders a marker and
  nothing else. A style built anywhere but the builder keeps the default palette
  and fails here.
- `TestTheSplashLockupWearsTheThemesRamp` -- the dashboard's emblem paints exactly
  the three ramp stops, under the default chrome and under an applied theme.
- `TestTheFilledStylesClearTheOnFillFloorInEveryTheme` and
  `TestTheRedesignKeepsTheBodyFloorOnEveryRoleItDrawsIn` -- the contrast the new
  surfaces draw with: every style that fills a block is measured ink-on-fill at the
  3:1 a bold run needs (for every shipped theme and for the default chrome), and the
  body ink, the key tone and the muted role are held to the 4.5:1 / 3:1 ladder the
  drift guard already uses.

**The contrast read.** The filled pairs pass in every theme: the on-fill ink is
chosen by `themeOnFillInk` and the guard measures the styles, not just the fields.
The body ink clears 4.5:1 in every theme; the key tone does too. The muted role
sits at 3.24:1 on the shipped default on purpose (it is metadata, and darker slate
would stop reading as metadata), which is why its floor is the 3:1 ladder and not
the body floor. **The one pair that does not clear a text floor is the marker's
ink**: `RowBarMarkerStyle` draws the accent on the brand fill -- 1.29:1 on the
shipped default, 1.3-1.9:1 across the seven themes -- and it is left as it is,
because it paints glyphs and never a word (the cursor's `▸`, the setup table's
chevron, the checklist's state glyph, the picker's `●`), and every word drawn on a
bar goes through `RowBarStyle`, which clears the floor. If the marker is ever meant
to carry text, the fix is a palette decision the mockup already answers (its bar
labels are all on-fill ink) and not something this pass could settle by itself.

**L6's status-dot follow-up, re-read and kept.** `Ready` and `Done` take the
success role, `Installing` the brand, `Preview mode` the accent, `Failed` the error
-- four roles for five states, every one of them measured by the sweep -- and the
word beside each dot is body ink, so the tone is decoration and the 4.5:1 floor is
carried by text.

**What did not move.** No golden and no layout: the ramp is colour-only and the
goldens that carry escapes do not draw the dashboard, the removed title branch
emitted the same bytes `BrandStyle` now does, and the two removed styles were
referenced by nothing outside `styles.go`. The twelve-terminal fit matrix is
unchanged.

**Pending.** The VHS captures were not re-recorded in this slice (the short loop is
the focused tests and the goldens; `make screens` runs with the PR). The
companion's possession ink is the one literal colour left in the package --
`companionEmberColour`/`companionHornColour` in `companion.go` and the one-row
art's `CompanionEmberStyle` (`#ff2d2d`) -- and it is outside both the style builder
and the redesign's surfaces: the possession is drawn as a sprite the theme has no
opinion about, which is what that code's own comment says, and a palette for it is
a creature-art decision rather than a theme one. `TitleStyle` and `DangerStyle` are
still unreferenced (above).

L18. User supplied four new reference screens and made them the primary visual
reference: Home, Utilities, Updates and Shell Startup
(/tmp/pi-clipboard-344905b0-cf83-4c41-89d5-585ee20013e1.png,
/tmp/pi-clipboard-563ff061-2bcf-40e8-9ced-1009de3220b2.png,
/tmp/pi-clipboard-950f6c50-6f0f-4f06-b0e6-2a158fc37c01.png,
/tmp/pi-clipboard-43e31190-8c48-4fc0-97ed-a6d9643f7bd5.png). Decision: reproduce
them as faithfully as Bubble Tea/Lip Gloss allow on top of the current work, keep
every function, key and real datum, keep the theme roles, the 80x24 usability and
the identity (pixel diamond, wolf, violet, retro), and extend the same language to
every other screen. The shared grammar they add: a filled chip label plus a
right-aligned caption on every card, a rule under it, rows with icon/title/subtitle
and a chevron, filled selection bars with a marker, numbered steps, bordered stat
boxes, action buttons that carry their key, fact rows with icons, grouped tables
with a scrollbar, resource bars, the warning badge, and a footer that keeps Ready
plus a centre status plus the key hints. The audit's P1 functional fixes remain
valid and are folded into this pass; the audit's P2 cosmetics are superseded.


L19. The four reference screens are reproduced on the current branch, and the
audit's P1 functional fixes are folded in.

**The shared grammar is code now** (`components.go`): `card` (a filled chip on the
top border, a right-aligned muted caption, a rule under them, then the body),
`cardRowLines` (icon, title, subtitle, right chevron; the selected row is one
filled bar with the marker and on-fill subtitle), `numberedStepLines`, `statBoxLines`,
`actionButtonRow`/`actionButtonRowAll` (each press carries its key; the primary is
filled), `iconFactLines` (a glyph, a label at a shared column, a wrapped value),
`resourceBarRow` (filled bar, hatched empty part, the value and percent),
`warningBadge` (a bordered amber box) and `groupedTableHeader`. They are built
only from the palette's roles (`CardChipStyle`, `StepNumberStyle`, `BadgeStyle`,
`OnFillMutedStyle`, `TableHeaderStyle`, all rebuilt by `applyUIColors`). The
footer's status row now carries an optional centred line (`footerCenterStatus`).

**Home** (`home_view.go`): the reference's two columns, the lockup and wordmark,
`Your personal development environment.` / `Fast. Consistent. Yours.`, the four
machine cards, the greeting, the environment sentence, the filled
`▶ Press [s] to start installation` and `or explore the menu with [Tab]`, and at
the right `Your machine`, `Next steps` (the real plan grouped by `InstallStep.Area`)
and `Recent backup` (the newest backup, or nothing). The 80x24 floor keeps the
compact dashboard. Tab now opens the menu, as the reference's footer says.

**Utilities** (`utilities_cards.go`): the card of two-line rows with its own
subtitle and chevrons, windowed with `↑ N more above` / `↓ N more below`, beside
`Theme and System Styling` (the real switch availability and the amber badge with
the section's own reason), `System Overview` (the real facts and the selected row),
`Resources` (the sampler's own CPU/memory/disk) and `Recommended next actions`
(only jobs the handler really offers). The compact card stays at the floor.

**Updates** (`update_center_view.go`): the stats strip, the filter pills, the
grouped table with a checkbox column, the `Package / Current → New version /
Manager / Scope` header, a scrollbar, the selected row filled, the focused entry's
card (`Installed`, `Candidate`, `Held`, `Note`, `Read with` — facts the table row
does not carry, so the detail no longer repeats the row) and the real keys. The
floor keeps the section renderer.

**Shell Startup** (`shell_audit_view.go`): the clock art, `The shell's startup`
with the numbered what/how many/controls rows, the filled `[s] Measure the shell's
startup` beside the real `[m]`/`[n]`/`[w]`/`[b]` options, `Results` with stat boxes
and the per-run bars and min/max/average/median, `Recent runs`, `Methodology` and
the footer's own `Median … over N of M runs` status.

**Functional fixes.** Escape on Setup keeps `Choices` and `SetupCursor`; the
utilities overview names the selected row; the section header's `Showing X-Y of Z`
is recomputed after the list band is trimmed, so it cannot advertise a row the
frame dropped; the update detail states only facts the row lacks; the theme
palette says `Showing: the applied theme` or `the previewed theme (press Enter to
apply)`; the installing floor names the checklist rows it hides and the step
counter is derived from the same progress as the bar; Escape quits the Complete
and Error screens; the doctor/update-center rows wrap onto a second line where the
list band has room and keep the one-line rows where it does not.

**Deviations kept deliberately.** The machine/plan/backup cards use the model's
real names (`Preparation`, `The extras`, …) rather than the reference's invented
copy; the update detail has no invented package description or install date; the
shell results show the model's samples (no per-run timestamps), and `Previous
median` appears only when the stored history has one; the wide layouts are gated
by row budgets, so a short frame falls back to the existing compact renderer; the
creature is still placed by the frame, not by the home body.

**Goldens.** `TestDashboardWideGolden`, `TestUtilitiesWideGolden`,
`TestShellAuditGolden`, `TestInstallingScreenGolden`, `TestInstallingLiveGolden`,
`TestInstallingTruecolorGolden`, `TestThemePickerWideGolden` and
`TestWelcomeLivePanelGolden` were regenerated; `TestShellAuditWideGolden` and
`TestUpdateCenterWideGolden` are new.

**Still to extend.** The reference grammar has not yet replaced the section
renderer used by the doctor, the other utility reports (WSL resources, terminal
capabilities, network, ports, SSH, disk cleaner, install verify), the wizard's
pickers, the Learn/keymap tables, the trainer and the backup/restore screens:
they keep the existing card/list grammar. Their rows and keys are unchanged; they
are the next slices, not regressions of this one.

L20. The first wizard picker moved onto the grammar: the OS question is drawn as
the reference's card (`os_select_view.go`), and nothing else about the screen
changed.

- **The page is the card.** The screen's own title — minus the `Step N:` prefix the
  header's meter owns — is the chip set into the card's top border, and the
  screen's own description (`Detected: Linux`, or the undetected sentence) is the
  caption at the right of the same row. There is no title line above the card any
  more: it was the same words twice once the chip carried them, which is how the
  section screens already draw (`sectionWideTitle`).
- **The answers are grammar rows.** `m.GetCurrentOptions()` in the order the key
  handler reads them, each as icon, title, muted subtitle and chevron, with the
  cursor's row filled and marked (`cardRowLines`). The subtitles name the package
  manager each platform is installed through — Homebrew and the Apple toolchain,
  the distribution's own manager, `pkg` on Termux — because that is the only thing
  the choice changes about the run; no fact about the host is invented.
- **The caption cannot be lost.** `card` gives its caption back when the border
  row has no room for it, and on an undetected host that sentence is the screen's
  only fact, so the card moves it inside the body instead of dropping it (80 wide,
  the undetected sentence, is the case).
- **The floor keeps a fallback.** The card is drawn when the frame can hold it —
  a content width of 32 columns and a row budget that holds the chrome plus every
  answer — and the compact list is drawn below that (36 columns, or 80x12), which
  is the same answers in one line each. The gate reads the frame only, never the
  cursor, so the shape cannot change while the user walks the answers.
- Unchanged, deliberately: the recorded answer (`Choices.OS`), the option labels
  and their order, Enter/Esc and the arrows, the plan panel beside the card, the
  wizard's `1/2` meter, and every other picker, which keeps the compact list until
  its own rows have subtitles.

Guards: `TestOSSelectDrawsTheReferenceGrammar` (the chip, the caption, the closing
border, the rows, the legend at 80x24 and 160x50),
`TestOSSelectRowsAreTheModelsAnswersInOrder` (the model's answers in order, one
marker, on the cursor's row), `TestOSSelectLeavesEnterUnnamedWithNoRowUnderTheCursor`,
`TestOSSelectFallsBackToTheCompactListWhenTheCardCannotFit` and
`TestOSSelectKeepsTheDescriptionWhenTheBorderCannotHoldIt`. Goldens:
`TestOSSelectGolden` regenerated, `TestOSSelectWideGolden` new.

L21. The terminal question moved onto the grammar, and the card it draws is now
one shared piece rather than the OS screen's own.

- **The card and the gate are shared** (`components.go`): `questionCard` (the
  chip, the caption, the rule, the groups of rows, and the caption moved inside
  when the border row has no room for it), `questionCardGroups` (one grammar row
  per answer in the option list's own order, the option list's separator drawn as
  the card's divider, the cursor's row filled and marked), `questionCardWidth`
  (the width the card is drawn in and the content measure its rows get) and
  `questionCardBody` (the gate: a readable content width and a row budget that
  holds the composed card). `cardWidth`/`cardInner`/`cardBorderCarriesCaption`
  are the one arithmetic `card` and the caption test both read, so the drawing and
  the decision cannot disagree. The OS screen keeps its own glyphs, subtitles and
  minimum width and calls the same four functions; its output is unchanged.
- **The terminal question draws the same card** (`terminal_select_view.go`): the
  screen's own title minus its `Step N:` prefix as the chip, its own
  description as the caption, every answer as an icon/title/subtitle/chevron row,
  and `Learn about terminals` in the group the option list's separator divides
  off. The subtitles are short phrases from the tool pages the installer already
  ships (`tools_info.go`) for the four terminals, `Install no terminal` for
  `None` (the run really does skip the terminal step) and `Each terminal's pros
  and cons` for the shortcut row; every one is under the 50 columns the left
  column leaves a row at the 124-column floor, so the row is never cut.
- **Unchanged, deliberately:** the recorded answer (`Choices.Terminal`), the
  option labels and their order (the macOS list still adds Kitty, WSL still shows
  the skip note), the separator, Enter/Esc and the arrows, the plan panel, the
  `1/2` meter, the Ghostty warning detour, the compact fallback and every other
  picker.
- **The note keeps its words.** The terminal question is the one whose
  description is two sentences (the WSL skip, the Alacritty build cost), and the
  card's border row cannot hold either beside the chip; the shared card moves the
  note inside itself, wrapped, instead of giving it back.
- **The floor still falls back.** The card is drawn when the frame can hold it
  (content ≥ 32 columns and the rows to spare); 36 columns wide or 80x12 draws
  the compact list, which is the same answers in one line each.

Guards: `TestTerminalSelectDrawsTheReferenceGrammar` (the chip, the caption, the
rows with their subtitles and chevrons, the legend and the state at 80x24 and
160x50), `TestTerminalSelectRowsAreTheModelsAnswersInOrder` (the answers in order,
the separator as the card's divider and not a row, one marker on the cursor's row),
`TestTerminalSelectRowTableCoversEveryAnswer` (a one-cell glyph and a subtitle
inside the row's measure for every answer the screen can offer),
`TestTerminalSelectLeavesEnterUnnamedWithNoRowUnderTheCursor`,
`TestTerminalSelectKeepsItsShapeWhileTheCursorMoves`,
`TestTerminalSelectFallsBackToTheCompactListWhenTheCardCannotFit` and
`TestTerminalSelectKeepsTheNoteWhenTheBorderCannotHoldIt`. Goldens:
`TestTerminalSelectGolden` and `TestTerminalSelectWideGolden` are new. The OS
guards and both OS goldens pass unchanged.

L22. The shell question moved onto the same shared card, and the slice is the third
picker the grammar draws.

- **The shell question draws the shared card** (`shell_select_view.go`): the
  screen's own title minus its `Step N:` prefix as the chip, its own description as
  the caption -- kept inside the card at both goldens, because the note names the
  detected shell and is longer than the border row can hold beside the chip -- every
  answer as an icon/title/subtitle/chevron row, and `Learn about shells` in the
  group the option list's separator divides off. It calls `questionCard`,
  `questionCardGroups`, `questionCardWidth` and `questionCardBody`
  (`components.go`); no gate or row shape is copied, and the card the OS and
  terminal questions draw is the one this question draws.
- **The subtitles claim only what the run does.** Fish carries the Fisher bootstrap
  the shipped `config.fish` performs, zsh the Oh My Zsh, Powerlevel10k and plugin
  packages the `.zshrc` sources, nushell the `config.nu`/`env.nu` this installer
  writes, and the shortcut row says it opens each shell's pros and cons; every one
  is under the 50 columns the left column leaves a row at the 124-column floor, so
  the row is never cut.
- **Unchanged, deliberately:** the recorded answer (`Choices.Shell`), the option
  labels and their order, the separator, Enter/Esc and the arrows, the plan panel,
  the `1/2` meter, the compact fallback and every other picker.
- **The floor still falls back.** The card is drawn when the frame can hold it
  (content ≥ 32 columns and the rows to spare); 36 columns wide or 80x12 draws the
  compact list, which is the same answers in one line each.

Guards: `TestShellSelectDrawsTheReferenceGrammar` (the chip, the caption kept
inside the card, the rows with their subtitles and chevrons, the legend and the
state at 80x24 and 160x50), `TestShellSelectRowsAreTheModelsAnswersInOrder` (the
answers in order, the separator as the card's divider and not a row, one marker on
the cursor's row), `TestShellSelectRowTableCoversEveryAnswer` (a one-cell glyph and
a subtitle inside the row's measure for every answer the screen can offer),
`TestShellSelectLeavesEnterUnnamedWithNoRowUnderTheCursor`,
`TestShellSelectKeepsItsShapeWhileTheCursorMoves`,
`TestShellSelectFallsBackToTheCompactListWhenTheCardCannotFit` and
`TestShellSelectKeepsTheNoteWhenTheBorderCannotHoldIt`. Goldens:
`TestShellSelectGolden` and `TestShellSelectWideGolden` are new. The OS and
terminal guards and their goldens pass unchanged.

L23. The font question moved onto the same shared card, and the slice is the fourth
picker the grammar draws.

- **The font question draws the shared card** (`font_select_view.go`): the screen's
  own title minus its `Step N:` prefix as the chip, its own description as the
  caption -- kept inside the card at both goldens, because the note is the step's
  measurement of this machine and is longer than the border row can hold beside the
  chip -- and both answers as icon/title/subtitle/chevron rows with the cursor's row
  filled and marked. It calls `questionCard`, `questionCardGroups`,
  `questionCardWidth` and `questionCardBody` (`components.go`); no gate or row shape
  is copied, and the card the OS, terminal and shell questions draw is the one this
  question draws.
- **The subtitles claim only what the run does.** The install row names the face the
  step puts on the machine (the Nerd Font patch of Meslo, the family the pinned
  archive declares) and wears the setup table's own font glyph; the skip row says
  the run leaves the fonts already on the machine alone, which is true whether the
  family is installed or not. Both are under the 50 columns the left column leaves a
  row at the 124-column floor, so the row is never cut.
- **The row table follows the model's labels**, because the install label carries the
  family's name and the step owns it: `fontSelectRowParts` is keyed by
  `m.fontOptions()` rather than by literals, so a face rename cannot leave the row
  decorated with another answer's sentence, and a Termux host (whose family is
  `MesloLGS Nerd Font`) draws the same row.
- **The note's paragraph break becomes a space**, and only there: the step writes its
  measurement and its consequence on two lines, and the shared card wraps a caption
  as one block, so a kept line break made the wrap land mid-sentence. The words are
  the model's own; the break is the frame's line breaks doing the work.
- **Unchanged, deliberately:** the recorded answer (`Choices.InstallFont`), the
  option labels and their order (`Install <family>` then `Skip it`), there being no
  separator row and so no card divider, Enter/Esc and the arrows, the plan panel, the
  `1/2` meter, the compact fallback and every other picker.
- **The floor still falls back.** The card is drawn when the frame can hold it
  (content ≥ 32 columns and the rows to spare); 36 columns wide or 80x12 draws the
  compact list, which is the same answers in one line each.

Guards: `TestFontSelectDrawsTheReferenceGrammar` (the chip, the description on the
border or kept inside the card, the rows with their subtitles and chevrons, the
legend and the state at 80x24 and 160x50),
`TestFontSelectRowsAreTheModelsAnswersInOrder` (the two answers in order, no divider
the question has no groups for, one marker on the cursor's row),
`TestFontSelectRowTableCoversEveryAnswer` (a one-cell glyph and a subtitle inside the
row's measure for every answer, on a host nothing measured, a Linux, a Termux and a
macOS host), `TestFontSelectLeavesEnterUnnamedWithNoRowUnderTheCursor`,
`TestFontSelectKeepsItsShapeWhileTheCursorMoves`,
`TestFontSelectFallsBackToTheCompactListWhenTheCardCannotFit` and
`TestFontSelectKeepsTheNoteWhenTheBorderCannotHoldIt`. Goldens:
`TestFontSelectGolden` and `TestFontSelectWideGolden` are new. The OS, terminal and
shell guards and their goldens pass unchanged.

L24. Five defects the user reported against the redesign, all in the tree
(uncommitted). Each one names what was wrong, what changed and the guard that
holds it.

- **The body is centred again, and the anchor is stable.** `bodyAnchorsToTheRule`
  (whose real name was `anchorBodyToTheRule`) moved every screen's spare rows from
  above the body to below it, so each screen started on the frame's first row and
  the space collected in one void above the creature (`view.go`). `placeBody` no
  longer caps its margin at six rows either: the cap was added for a body that
  *floated*, and the anchor replaced it with a body that hugged the rule. The body
  is now centred in the rows between the header and the footer, and on a screen
  whose creature block is reserved (`panelsOfferPlan`) it is centred in the rows
  above that block (`placeBodyAboveCompanion`), so the pet keeps the rows the
  terminal gave it and the 80x24 floor keeps its five-row shift. That block is
  reserved with the creature's clock ignored (L31), so the body is centred in the
  same rows whether animation is on or off.
- **Two things had to be true for centring to be safe, and both are fixed.**
  A body that changes height as the cursor walks it moves itself, so: the section
  screens' prose card now holds the rows of its column instead of ending at its
  text (`sectionWideDetailCard`) -- the disk cleaner's prose follows the cursor and
  moved the frame by three rows -- and the setup screen reserves the Space hint its
  footer gains when the cursor lands on a switch row (`selectionHintReserve`,
  `renderSetup`), because taking that row from the body changed the shape the table
  was drawn in.
- **The utilities theme card no longer denies its own row.** The card carried an
  amber "Not available on this host" badge and a paragraph about GNOME/KDE/macOS
  because no DESKTOP session was detected under WSL, while the row above it opens
  the installer's own theme picker, which works either way. The badge is gone; the
  card states the picker's own state (`utilitiesThemeState`: how many themes were
  read and which one is worn, or the reason the list cannot be read) and names the
  desktop switch in the section's own sentence (`utilitiesDesktopSwitchNote`), so
  the absence is still declared and nothing claims an availability the model does
  not hold. `warningBadge` stays in the grammar with no caller and a comment saying
  why.
- **The update center's list scrolls inside its card.** The table's body was
  measured without the two notes it grows at the ends of its window, so a windowed
  list drew the frame two rows past the terminal's height: the terminal scrolled,
  the header left the screen and rows disappeared. The card is now given the rows
  the stats strip left (`updateCenterTableColumn`) and holds both note rows whether
  or not the window draws them (`updateCenterTable`), so the header, the stats, the
  table's heading and the footer never move and only the window does. The detail
  card is clipped to the same budget rather than growing the body, and the note is
  still the truth about what the window hid.
- **The shell startup's clock and its primary press.** The clock art mixed `▐` and
  `▀▀` into a ring of `█` with its hand one column off the centre, so it read as
  broken pixels; it is one symmetric ring with a hand up and a hand right, every row
  the same width. The `[s]` measure press was one filled block the height of its
  box, which is this grammar's word for *the row under the cursor*, so the call to
  action read as a selection bar while the marker sat in the list; it is now the
  bordered box its neighbours wear, painted in the brand role (`actionButtonOf`).
  The key, the press and the plan options are untouched. The wide body also takes
  the frame's leading margin now, so a terminal wider than the reading measure does
  not leave the whole screen hugging the left edge.
- **The install review draws the shared grammar.** The screen was the last one
  still on the bare bands: a title, a description and one-line bars. It is a card
  now (`installReviewCardBody`): the screen's own title as the chip, its own
  description as the caption (kept inside the card when the border row has no room,
  which is what `questionCard` already does), one group per area with the area on
  the card's divider, one row per unit with the area's glyph in the gutter, its
  `[x]`/`[ ]` mark at the head of the title and its provenance on the muted line
  under it, the cursor's row filled across the card, the count summary under a
  blank row and the way-back block under the card in the rows it reserves. The rows
  carry no chevron on purpose: a unit is switched where it stands and this grammar
  spends the chevron on the rows that open something. The panel beside it is the
  same plan panel it always was. The toggles, the summary, the keys and the compact
  fallback are unchanged; the list still reports what its window hid.

Guards. `TestPlaceBodyCentresInTheRowsBetweenHeaderAndFooter` (the three reviewed
sizes, replacing the cap guard), `TestTheCentreDoesNotMoveWhileTheCursorWalks`
(fourteen screens across the measured terminals: the body starts on the same row at
every cursor position), `TestUpdateCenterScrollsInsideItsCard` and
`TestUpdateCenterWindowNoteIsTruthful`, `TestTheThemeCardSpeaksForTheThemeTheRowOpens`,
`TestTheClockArtReadsAsAClock`, `TestTheMeasurePressIsAButtonAndNotASelectionBar`
and `TestThePlanScreenIsDrawnAsTheReferenceCard`. Two guards read a row's *name*
instead of its one-line text, because the card splits the row into a title and the
line under it: `TestThePlanScreenShowsThePlanByAreaInTheRunsOrder` and
`TestTheScreenFollowsTheCursorAndReportsWhatItHides`. Goldens: every golden of the
screens the centring moved was regenerated (33 files), including the four reference
screens' wide frames.

Two things found and not taken. `TestSinglePanelColumnIsThePanelsOwnRows` and
`TestTabRowNamesThePanelsAndMarksTheActiveOne` are red on this branch and were red
before this slice: both compare `panelCaption`'s output against per-name escape
sequences, and `card()` renders the caption (names joined by ` · `) as one styled
string. An action row under the update center's list still scrolls the window back
to the first entry, because the table scrolls to the entry under the cursor and an
action row has none; keeping the last window needs a field the model does not have.
At the 80x24 floor the review's card shows three or four of a sixteen-unit plan
with "↓ N more below", which is the grammar's two-line row cost; the compact
one-line fallback is kept for frames too narrow for the card.

**The multiplexer and the editor join the reference card.** The last two wizard
questions still drew the shared menu card with one-line answers. They are the
question card now (`wm_select_view.go`, `nvim_select_view.go`), through
`questionCard`/`questionCardBody`/`questionCardGroups` like the four before them:
the screen's own title as the chip, its own description as the caption -- inside the
card where the border row has no room, which is what the editor question does in the
two-column frame -- the rule under the pair, and every answer as an
icon/title/subtitle/chevron row with the cursor's row filled and marked. The option
list's separator is the card's own divider, so the shortcut rows sit in the group the
card sets off. Nothing the screen decides changed: the answers are
`GetCurrentOptions()` in its order, the labels, the recorded choices, the keys, Enter
and Esc are untouched, and the footer is the same legend. The subtitles come from the
tool pages the installer already ships (`GetWMInfo`, `GetNvimInfo`); `None` and `No`
say only what pressing them does. The multiplexer list stays the host's -- a Termux
host draws three answers and no Herdr -- and the compact fallback is kept for a frame
too small for the card. This closes the follow-up above about the pickers' one-line
answers.

Guards. `wm_select_view_test.go` and `nvim_select_view_test.go`: the grammar's shape
at 80x24 and 160x50, the answers in the model's order with the divider and the one
marker, a one-cell glyph and a <=50-column subtitle for every answer, no Enter
advertised with no row under the cursor, the same top border at every cursor
position, the compact fallback at 36x24 and 80x12, and -- for the multiplexer -- the
Termux host's three answers and no fourth. Goldens: `TestWMSelectGolden` and
`TestWMSelectWideGolden`, `TestNvimSelectGolden` and `TestNvimSelectWideGolden`.

**The Learn pages, the keymap tables and the LazyVim guide join the reference
grammar.** They were the last pages still drawing the old vocabulary: a bold title
row, a description sentence under it, a blank row, and then floating lines. They are
cards now (`reference_cards.go`), through the shared `cardGroups`/`cardRowLines`
grammar every other screen uses:

- **The menus** -- the Learn hub, its four tool menus, the prompt menu, the Vim hub,
  the five keymap menus and the LazyVim guide menu -- draw `menuCardBody`'s card:
  the page's own title as the chip, its own description as the caption, the rule
  under the pair and `GetCurrentOptions()` as one row each, with the option list's
  separator as the card's divider and the row under the cursor filled and marked. A
  row that opens a page wears the chevron; the way back acts where it stands, so it
  keeps its arrow and no chevron. The Learn hub's two sections stay named: the first
  on the chip (a divider only precedes a group, and the first group has none above
  it) and the second on the divider, from the two constants `learn_menu.go` now
  holds. The hub takes the section's name on the chip only while its whole list fits
  under it, and that decision reads the list, never the cursor.
- **The tool pages** -- terminals, shells, multiplexers, the font, Neovim and the
  prompts -- are `toolInfoCardBody`: the tool's own name as the chip, its website as
  the caption, its description as the card's prose and `info.Pros`/`info.Cons` under
  the `✓ Pros` and `✗ Cons` dividers, each reason wrapped whole with the note that
  counts the ones the rows could not hold. The old body is kept as the fallback for a
  frame too narrow or too short for the card.
- **The read-only tables** -- shell tooling, plugins, the machine toolset and the
  WezTerm/Alacritty settings -- are `referenceTableCard`: the table's own name as the
  chip and its description as the caption, with each entry as the grammar's row (the
  name as the title, its kind trailing at the right edge, the sentence whole under
  them).
- **The keymap tables** keep their three columns, which are the page, and take the
  grammar's header role above them; the category's own rows are built by
  `keymapCategoryTable`, which the scroll keys read too.
- **The LazyVim topic** is `lazyVimTopicCard`: the topic's title as the chip, its
  description as the caption and the window of its lines under the rule, the example
  and the tips keeping the marks they had.

No datum moved. The rows are the pages' own lists in their own order, the labels,
the keys, the recorded choices and the footer legends are untouched, and the tables
keep every row. The card's chrome is three rows where the old head was three rows --
its top border, the rule under the chip and its bottom border -- so every page keeps
the rows it had and its scroll keys keep their distance. A caption the border row has
no room for moves under the rule (`referenceCard`), and its rows come off the window
first, so a narrow frame draws fewer rows instead of one row past its budget; the
budget the keys clamp with is the budget the card spent (`referenceRowBudget`,
`lazyVimTopicRows`, `keymapCategoryRows`). Two small repairs came with it: the prompt
menu had no screen title at all, so its card opened under a blank heading, and a
caption could be dropped when its card had no chip to sit beside -- both now say what
they hold. The palette is the applied theme's roles; nothing here names a colour.

Guards. `reference_cards_test.go`: every converted page draws the shared card with
its own chip and its own rows inside it and fills the frame exactly, the footers
still advertise the keys the handlers answer to, the tool pages keep both lists and
wrap their reasons, a keymap table's card spends exactly its window, and a LazyVim
topic's card is its window plus its chrome. The existing guards were re-pointed where
the shape changed: `TestLearnTablesDrawEveryEntryWhole` reads the entry renderer
directly, `TestKeymapCategoryLastBindingIsReachable` asks `keymapCategoryRows`, and
`TestHubCardStaysWhereItIsWhenTheCursorMoves` compares the rows of the two cards'
tops and bottoms separately, because the body card's top now shares a row with the
panel's. Goldens: `TestLearnHubGolden`/`TestLearnHubWideGolden`,
`TestVimHubGolden`/`TestVimHubWideGolden`,
`TestLearnToolGolden`/`TestLearnToolWideGolden`,
`TestLearnTableGolden`/`TestLearnTableWideGolden`,
`TestKeymapTableGolden`/`TestKeymapTableWideGolden` and
`TestLazyVimTopicGolden`/`TestLazyVimTopicWideGolden`. The review capture
(`capture_redesign_test.go`) lists the Vim hub, the prompt page, the keymap menus,
the long-key Herdr table and the LazyVim guide and topic beside the Learn pages it
already had.

L25. The flow's four screens -- the backup question, the two restore screens, the
completion and the failure -- moved onto the same shared grammar, and they were the
last pages drawing the older panel vocabulary: a branded label row, a guttered block
of prose and a separate bordered box for the answers, which is three objects where
every other screen now draws one. They are `flow_cards.go`, through `card`,
`cardGroups`, `cardRowLines`, `iconFactLines` and `questionCard` -- the same
functions the six questions, the review, the reference pages and the section screens
call, not a second vocabulary.

- **The backup question** is one card: the screen's own title as the chip, its own
  sentence ("The following configs will be overwritten:") as the caption -- inside
  the card where the border row has no room for it, which is what 60 columns does --
  `m.ExistingConfigs` as rows with the mark the compact list always drew, the note
  that says why the backup exists under the card's rule, and the three answers under
  the next. A machine with more configurations than the rows allows keeps the note
  that counts the ones it could not show.
- **The restore history** is one card too: the backups with their timestamp and file
  count as rows, the same window `listWindow` always gave the list, and the same
  `← Back` behind the card's divider. The scroll range stays the header's vital sign
  and not a body row, so the way back cannot move as the history grows.
- **The confirmation** names which backup it is in the caption (the backup's own
  timestamp), the files it carries under the card's `Contents` divider, the warning
  under the next and the three answers under the last. `Contents` is the word the
  contents card already carried, so the divider is named and no word is new.
- **The completion** states the same recorded answers as the card's fact rows, with
  the glyph the setup table draws for the same decision, and the next step under the
  card's own rule -- or, in a dry run, what the run did not change, with the same
  rehearsal notice as the caption. `completionFacts` and `completionShellCommand`
  are one source for both shapes, so the compact body and the card cannot state two
  different records.
- **The failure** is the message, the way back under a divider named for it and the
  freshest log lines under a divider of their own. The message keeps its cap and the
  way back is charged for before the log, which is the order the screen always drew
  them in.

Nothing moved. The answers are `GetCurrentOptions()` in the key handler's order with
`m.Cursor`, the configurations, backup lists, file lists and log lines are the
model's own, the completion's values still come from `completionChoiceLabel` and
`fontSummaryValue`, and the failure's sentences are still `wayBackAfterAFailure`'s.
The palette is the applied theme's roles. Each renderer keeps the compact body it
always had under the frames the card cannot hold -- the gate is a content width and a
row budget and nothing about the cursor -- so the 80x24 floor and the two-column
frames both keep the shape they had.

Guards. `flow_cards_test.go`: every one of the five renderers sets its own chip into
the card's top border, draws the rule under it and every answer inside the card with
the cursor's row marked; every fact the screens state is still on them, at the floor
and at 140x44, in the screen's own words; and the gate accepts the 80x24 floor and
refuses a 30-column frame, where the compact body still renders. The matrix guard
(`TestEveryScreenFitsEveryTerminalSize`) passes for all 96 screens at all twelve
terminals, so the taller card does not overflow the 60x20 floor. Goldens regenerated:
`TestBackupScreenGolden`, `TestCompleteScreenGolden`, `TestCompleteTruecolorGolden`,
`TestCompleteCelebrationGolden` and `TestErrorScreenGolden`. The two restore screens
have no golden; they are held by the shape and data guards and by the E2E reaches
that already read their final frames.

L26. The utilities section stops announcing the desktop theme switch it does not
have, on the user's decision: the sentence *"No desktop theme switch is available
here: no GNOME, KDE or macOS session."* sat directly above the row that opens the
installer's **own** theme picker, which works under WSL and everywhere else, so two
"theme"s in one screen read as one verdict on the picker.

- **The sentence is gone from both bodies.** `utilitiesReasons` and
  `utilitiesDescription` append it when no switch was detected; both now append
  nothing in that case and keep the detected-desktop sentence unchanged. The theme
  card's note went with it: `utilitiesThemeCardBody` no longer calls
  `utilitiesDesktopSwitchNote`, and that function is deleted -- it existed only to
  copy the section's sentence into the card. Nothing claims a switch is present: the
  rows are drawn only when one was detected, so the absence is shown by the rows the
  user did not get, not by prose over the picker.
- **The panel follows the section.** `utilitiesPanelEntries` used to carry a
  `Switch` entry with `absent: "No desktop theme switch is available here."`; the
  entry is now added only when a switch was found, under the section's own condition.
  A panel line naming the absent DESKTOP switch had the same defect as the sentence,
  one column over.
- **The guards were inverted, not dropped.** `TestUtilitiesUnavailableIsAboutTheModelNotTheHost`
  and the frame arm now known as `TestUtilitiesUnavailableIsQuietAndFitsTheFrame`
  assert the aviso is absent while keeping what they were for: the first still makes
  the ambient detection find a switch so the assertions can only be about the model,
  the second still renders 80x24, 60x20 and 160x50 and fits each frame. The macOS
  case (`TestUtilitiesAreOfferedOnMacOSWithDefaults`) keeps its "a detected desktop
  must not be told it has no switch" assertion, `TestTheThemeCardSpeaksForTheThemeTheRowOpens`
  now requires the card to be silent about the desktop, and the panel guards
  (`TestUtilitiesPanelFactsAreDerivedFromTheModelState`, `TestMainMenuPanelDescribesTheOptionUnderTheCursor`)
  require the aviso to be gone from the empty model's panel and from the Utilities
  row. Goldens regenerated: `TestUtilitiesGolden` and `TestUtilitiesWideGolden`; no
  other golden carried the sentence.
- **Four frames nobody would have guessed depend on it.** `panelColumn` draws the
  main-menu card at the tallest its facts get over *every* row the cursor can be on,
  so the Utilities row's panel is the list that sets that height. One entry shorter
  took one row off the card, and the frames that hold it changed even though none of
  them says the sentence: `TestMainMenuWideGolden` and the three companion goldens
  (`TestCompanionGoldenFramesTheCreatureAtTickZero`,
  `TestCompanionGoldenPinsTheFullSpriteAndItsGaze`,
  `TestCompanionGoldenPinsThePixelSpriteAndItsGaze`). Each differs by that one
  interior row, moved to the composition's spare row by centring; the creature, the
  menu's rows and every other card sit where they sat. Verified the other way round
  too: with the three touched sources taken from HEAD, all four pass.

L27. The ports and the section's two reports moved onto the shared grammar. The
slice was scoped to five screens -- ports, SSH, WSL, terminal capabilities and
install verify -- and it converted the three that had a body the grammar had no
word for yet; the SSH and the WSL screens already draw the reference's cards
(section_wide.go, landed with the doctor and the network diagnosis) and their
remaining gap is recorded below as what this slice did not take.

- **The ports draw two cards** (`port_cards.go`). The manager is the reading in a
  card at the left and the reading's own facts in the card beside it
  (`portFactsChip`, the section's own word for a detail column): every socket is a
  `cardRowLines` row -- the port, protocol, state and address as the title, the
  owner and the bind's reach on the muted line under it, the chevron because Enter
  opens the socket's facts -- windowed with `utilitiesRowWindow` and the notes that
  name what the window hid, the reading's notice under the card's own rule, and in
  the right card the scope sentence, the filter/sort/refresh state and, once Enter
  has opened the row, `iconFactLines` of the selected listener's fields. The
  confirmation is one card: the account as prose under the chip and the answers it
  offers under the card's rule, each row still carrying its own letter.
- **Nothing the reading holds was dropped.** The facts are the listener's own
  fields -- port, family, state, source, owner and PID and account, executable,
  redacted arguments, start time, the identity caveat, and the reading's declared
  limitations -- plus the notice, the scope and the filter state. The redaction is
  the reading's, so the card prints the same text the copy sends.
- **The two reports draw rows** (`report_cards.go`). A capability is one row: its
  name as the title, the answer at the row's right edge (`cardRow.Trailing`), the
  source and consequence on the muted line under both, the state's mark in the
  gutter (`✓`/`✗`/`?`, the section's own three), and -- for an answer the probe
  could not determine -- the reason and the way to check by hand, wrapped whole by
  `wrappedCardRowLines` rather than cut. A verification finding is one row: the
  machine's verdict in the gutter, the path as the title, the configuration's own
  name at the right edge, and what happened to it (or the digest comparison) under
  both, grouped under the `missing` and `changed` dividers with the record's own
  sentence and the "still what the run left" count around them.
- **One source per sentence.** `terminalCapabilityDetail` and
  `installVerifyOutcome` are the sentence and the word both shapes use, so the
  compact report the floor draws and the card the wide frame draws cannot explain
  one answer two ways; `capabilityReadOnlyPromise` and
  `installVerifyReadOnlyPromise` moved into `report_cards.go` beside the rows they
  introduce.
- **The card's composition stayed where it was.** `sectionWideReportCard` keeps the
  chip, the rule, the way back and the notice and asks `sectionReportGroups` for the
  body only, so the two report screens changed shape and no other section screen
  did. `clipCardGroups` cuts a report's own rows first -- the way back is charged
  for before them -- and a group that gives way says how many rows it could not show.
- **A wrong fact came out with it.** The report screens printed
  `Showing 1-0 of 1` in the header: `renderSectionWide` measured a window over the
  rows a report does not have. A report now reports no range at all.
- **The floor keeps the compact renderer.** Both port cards and both report cards
  are gated on the frame -- two columns and a row budget, never the cursor, the
  filter or the notice -- so 80x24 draws exactly the body it drew before
  (`TestPortManagerGolden`, `TestPortStopGolden`, `TestTerminalCapabilitiesGolden`
  and `TestInstallVerifyGolden` are unchanged).

Guards. `port_cards_test.go`: both port screens draw the shared card at 160x50 and
no card at the floor, the manager's card states every socket with its owner and
reach, the scope, the filter state, the selected socket's facts and the notice, the
confirmation's card states the identity, every port it releases, the outcome and
the verification, the card's top border is the same at every cursor position, the
footer still names every key the handler answers to, and both screens fit all
twelve measured terminals. `report_cards_test.go`: the capability card draws one
row per capability with the model's own answer, source and consequence (and the
reason and manual check for an unknown one), the verification card draws every
finding with its path, name, outcome and comparison, both read the model rather
than a second copy (a changed answer and a no-record verdict are drawn as the model
holds them), the floor draws no card, and neither report advertises a window.
Goldens: `TestPortManagerWideGolden`, `TestPortStopWideGolden` and
`TestInstallVerifyWideGolden` are new; `TestTerminalCapabilitiesWideGolden` was
regenerated (its answers are rows now and its header no longer claims a range).

Not taken, and why:

- **The SSH screens and the WSL screens** already draw the reference's cards on a
  two-column frame: their rows are `cardRowLines` and their prose is the card beside
  them (`section_wide.go`, and `TestSectionWideDrawsTheGrammarsCard` holds the WSL
  resource screen in that list). What they do not have is the reports' row grammar
  for their *detail*: `sshHostDescription`, `sshManagerDescription`,
  `wslAdvisorDescription` and the resource screen's numbers are prose composed by
  their own description functions, and turning them into `iconFactLines` means
  rewriting each screen's description into facts and deriving the prose from those
  facts -- the way `terminalCapabilityDetail` does for the capability report -- so
  the compact floor and the card cannot drift. That is eleven screens' worth of
  content refactoring with a guard each, not a page's, and it is the next slice.
- **The action rows** of the section screens (the SSH and WSL screens included) are
  still rows inside the list rather than the grammar's presses in the card. That is
  the same ten-screen decision the previous entry records: it belongs to whoever
  owns the section renderer.

L28. The SSH screens and the WSL screens state their detail as facts. L27 recorded
them as the next slice: their cards existed and the detail inside them was prose, so
the label that names a fact and the value beside it were the same block of
sentences. They are now one list of `iconFact`s per screen, drawn by
`iconFactLines` in the card beside the list and by `factProse` in the compact body,
so the floor and the wide frame read one source. Eleven screens, no other screen
touched.

- **One registry, two shapes** (`facts_cards.go`). `sectionScreenFacts` answers for
  the eleven screens -- the seven SSH screens (`sshManagerFacts`, `sshHostFacts`,
  `sshFormFacts`, `sshKeyFacts`, `sshAgentFacts`, `sshDiagnosticFacts`,
  `sshTestFacts`) and the four WSL screens (`wslResourceFacts`, `wslAdvisorFacts`,
  `wslBenchmarkFacts`, `wslMigrationFacts`) -- and every screen's own
  `...Description()` is now `factProse` of that list. `sectionWideDetailCard` asks
  `sectionScreenGroups` first and draws `factLines` of it; a screen that is not in
  the registry keeps its prose untouched, so the registry is the whole gate and no
  other section screen changed shape.
- **The prose keeps its own sentences.** `factProse` reads a value that is already a
  sentence as it stands, introduces a bare value with the label that names it
  ("Host name: web.example.com  (the client's answer)."), and prints a block under
  its label one line per paragraph. That is what let the screens keep the copy they
  were written with: the compact bodies are the same sentences the floor drew,
  which is why the two WSL floor goldens did not change. The labels are new and
  they are the only new words: they name the fact ("Files", "Hosts", "Declared",
  "Memory guard", "Evidence", "Space", "Placed") and never state one.
- **A block is a block.** A preview, a public key and the client's own output carry
  their own lines, and wrapping them as one sentence would join lines that mean
  different things. `factLines` splits those facts out and draws them with the label
  as the heading and every line at the value column; `factProse` prints them one
  paragraph each, in the order the plan, the key or the client wrote them. The
  preview's leading spaces survive, so a diff still reads `- …` against `+ …`.
- **Nothing was dropped.** Every datum is a field of the model or the screen's own
  sentence: the SSH manager's files, host counts, Match blocks, read failures, tool
  versions and write policy; one entry's declared line, read-only reason, effective
  values with their source, options and public key; the form's plan, file, backup
  and diff; the key inventory, the selected key's fingerprint, encryption, warnings
  and public half, and what known_hosts holds; the agent's kind, socket,
  reachability, identities, notes and the ssh-add caveat; the diagnosis count and
  the finding under the cursor with its fix and location; the test's command,
  bounds, promises, outcome, explanation, arguments and bounded output; the WSL
  resource plan, file, guard report, the memory rule, the write's rules; the
  advisor's environment, read-only promise, unread limits and the finding's
  evidence, consequence, benefit and action; the benchmark's bounds, targets,
  caveat, results, clean-up and contamination; and the copy's every phase.
- **The measured facts stayed quoted.** `sshEffectiveRows` is now the prose of
  `sshEffectiveFacts`, so its provenance guards read the same list the card draws,
  and `wslEnvironmentParagraphs`, `wslFindingParagraphs`,
  `wslBenchmarkResultsParagraphs` and the five `wslMigration*Paragraphs` are the
  prose of their facts. The tests that pin a sentence -- the read-only reason, "A
  private key is never read by this screen", the memory fraction against
  `wslMemoryRatioDivisor`, the policy's own prose -- are unchanged and green.
- **The floor keeps the compact renderers.** The card is drawn only where
  `sectionWide` already held (two columns, a row budget), so 80x24 draws the body it
  drew before and `TestWSLResourcesGolden` and `TestWSLAdvisorGolden` are unchanged.
  One guard did move: `TestScreenWSLResourcesShowsTheHostNumbersAndTheRecommendation`
  now reads `wsl --shutdown` from the screen's own sentence instead of from the
  glyphs, because the card wraps a fact's value at its column and the command can
  fall across two rows.

Guards. `facts_cards_test.go`: every converted screen is in the registry, every
fact's label and value is in the card's body, every value reaches the compact body
whole, a datum changed in the model reaches both shapes and the value it replaced is
gone from both, the 80x24 floor draws no card and still opens on the first fact, all
eleven fit every measured terminal, and `factProse`/`factLines` keep the three
readings (sentence, labelled value, block). No golden was regenerated.

Not taken:

- **The card's rows still cut at the column.** A screen with more facts than its
  column has rows ends in the frame's cut marker (`… and N more`), which is the
  card's own contract; the capabilities and verification reports solve it by
  grouping and counting, and the SSH and WSL screens could too, one screen at a
  time. Nothing is lost from the model or the compact body.
- **The action rows** are still rows inside the list, and the wide goldens for these
  eleven screens are not pinned yet -- both belong to the section renderer's owner,
  as L27 records.

L29. Four defects the user reported against the wide cards, all four from one
convention: a card padded to the frame's row budget for stability. Each one is
fixed at its cause and held by a guard.

- **The update center's list card hugs its window.** The table was given every row
  the stats strip left, so a machine with seventy packages drew a table from the
  strip to the footer and the composition had nothing left to be centred in. The
  window is a reading measure now (`updateTableWindowRows`, twenty rows: about the
  table the reference draws), a longer list scrolls inside it, and the card is
  exactly the rows it draws -- the heading, the rule, the window and the two note
  rows, which are still drawn whether or not one of them has something to say.
  Nothing is padded, so the frame's spare rows stay above and below the
  composition. The detail's own card is measured the same way it is on the section
  screens (`updateCenterDetailColumn`): the tallest reading any visible package
  draws, so walking the list cannot change the composition's height either.
- **The cursor rests only on packages.** `up`/`k` and `down`/`j` move to the next
  package row (`updateCenterStep`), skipping group rules, the action rows and the
  way back -- the actions carry their own letters and the way back is Esc, so
  nothing became unreachable. And the detail no longer blanks: when a state puts
  the cursor on a row that is not a package, `updateCenterDetailItem` describes the
  nearest package above it, then below, and the table keeps its filled row on it.
  That also removes the follow-up L24 recorded: an action row no longer scrolls
  the window back to the first entry, because the window follows the entry the
  card describes.
- **The section "Details" card is capped at its tallest reading.**
  `sectionWideDetailCard` was padded to the column's whole budget, which is why a
  short reading left a frame-height card. To measure the reading without moving it,
  `renderThemeScreen` takes the screen's own description function instead of its
  paragraphs (`Model.doctorDescription`, `Model.diskCleanerDescription`, …), and the
  card asks that function for the detail at every row the cursor can rest on and
  holds the tallest. The frame's rows are still the ceiling, and the cut marker is
  unchanged. The port facts card is capped the same way
  (`portManagerFactsCard`): the head, the divider and the longest socket's facts,
  measured with the facts open, so neither opening the detail nor walking the
  reading resizes the column. The doctor's Details card went from 45 rows to 13 at
  160x50, the network diagnosis's from 45 to 15 and the port facts column from 45 to
  19; the disk cleaner's prose is longer than any column, so its card is still cut
  at the budget and its golden did not move.
- **The scrollbar tracks the window proportionally.** The thumb was placed around
  the middle of the visible window wherever the window sat, so the first screenful
  read as half way down the list. The track is one cell per drawn row and stands for
  the whole list: the thumb is the slice's share of it (rounded up, so a one-row
  slice still fills a cell) and begins where the slice's first row falls on it
  (`updateScrollbarCell`). It now starts at the top of the column and ends at its
  foot.

Guards. `TestUpdateCenterListCardHugsItsWindow` (the card is the same height at
160x50 and 160x80, shorter than the room it was given, has no blank row inside it,
and the composition is centred), `TestUpdateCenterCompositionHoldsStillWhileTheCursorWalks`
(both columns' top and bottom rows are the same at every cursor position),
`TestUpdateCenterCursorVisitsOnlyPackageRows` (walking down visits every package and
nothing else, with the detail card drawn at each stop), 
`TestUpdateCenterDetailFallsBackToTheNearestPackage` (an action row describes the
nearest package), `TestUpdateScrollbarFollowsTheWindowProportionally` (the first
rows fill the top cell, the last rows the last one, half the list fills half the
track and the whole list fills all of it, and the thumb only travels down),
`TestSectionWideDetailCardHoldsItsTallestReading` (every section screen's card holds
one height, one cursor position fills it exactly, and a short reading leaves a card
shorter than the body) and `TestPortFactsCardHoldsItsTallestReading` (the same, and
the same span with the detail open and closed). Goldens regenerated:
`TestUpdateCenterWideGolden`, `TestDoctorWideGolden`, `TestNetworkDiagnosticsWideGolden`
and `TestPortManagerWideGolden`. No other screen changed shape: the section screens
whose detail already ran past their column still draw the card at the budget, and
the 80x24 floor keeps the compact renderers it had.

Two things left. `sectionWideReportCard` -- the one card a report draws across the
composition -- still fills the body rather than its reading; the reports have no
list and no second column, so capping it is a separate decision about the report
screens. And the description function now reaches the screen renderer as a
parameter, so a new section screen must pass it (`renderThemeScreen(Model.xDescription, …)`)
rather than a slice; a screen that passed the wrong one would cap its card at
another screen's reading.

L30. The trainer's six screens moved onto the shared grammar, and the trainer
stopped spelling its state with emoji. This is the slice L6 and L19 named as
outstanding for the trainer.

- **Two shapes, one preference** (`trainer_cards.go`). Where the frame holds two
  columns and the rows the body needs, the six screens draw the reference's cards
  through `card`/`cardGroups`/`cardRowLines`/`numberedStepLines`/`iconFactLines`/
  `statBoxLines` — the same functions the sections, the review, the questions, the
  reference pages, the flow screens, the ports and the reports call, not a second
  vocabulary. Everywhere else (the 80x24 floor, the 60x20 floor, and any frame too
  short for the card) they keep the compact body they shipped with, so the four
  trainer goldens, `TestTrainerScreensFitTheFrame` over the whole corpus and the
  documented floor arithmetic are unchanged. The gate is
  `trainerCardsFit`/`trainerBodyBudget`: the frame and the rows, never the cursor.
- **The menu is two cards.** The modules are grammar rows (the state's glyph in the
  gutter, the module's name, its state word and its commands on the muted line, the
  two meters trailing) in a card captioned with the run's own counts, windowed
  through `utilitiesRowWindow` with the notes that say what it hid. The module
  under the cursor is a card of its own: the chip is the module's name, the caption
  its state word, the first group the three-rung ladder (lessons, practice, boss)
  with the recorded counts, the second the practice reading as fact rows. Every
  number is `ModuleProgress`, `GetPracticeStatsForModule`, `ReviewsDue` or the
  module's own `BossName`; nothing is invented and the card creates no records.
- **The playing screens are two cards.** `renderTrainerExercise` and
  `renderTrainerBoss` compose the code window in a `Code` card captioned with the
  rows on screen, and the mission in a card chipped with the screen's own word
  (`Mission`, `Challenge`) and captioned with the exercise's identifier, the answer
  under an `Answer` divider and the feedback area under the card. `trainerWideTextBudget`
  now subtracts the cards' own rows (`trainerCardChrome`, `trainerExerciseCardChrome`)
  and is still the one budget the renderer and the scroll keys read, so the window
  the keys move is the window the card draws. The boss joins the two-column
  predicate; its header chips (lives, step, countdown, combo) are untouched.
- **The result screens are a card.** The exercise result states what was answered
  as fact rows, the verdict's message, the explanation and, for a buffer-verified
  exercise, both buffers under their own dividers. The fight's result puts the run's
  numbers in stat boxes and the verdict, the measured fight time and the module's
  best under the `Summary` card's dividers. The compact heads keep their own copy:
  `trainerBossResultSentences` is one source for both shapes, so the floor and the
  card cannot say the outcome two ways.
- **No emoji, and the class guard now measures the trainer.** The trainer's own
  copy carried emoji in eight places: the boss header's crossed swords, the three
  celebration lines, the weak-exercise warning, the hint label's bulb, the reset
  prompt's warning, the erased-profile broom, the victory trophy and every handler
  message (`💀`, `⏰`, `🔒`, `🎉`, `🔁`, `🔄`, `✨`). They are Nerd Font glyphs
  (`\uf017`, `\uf021`, `\uf071`, `\uf0e2`, `\uf0eb`) or plain marks (`★`, `◆`,
  `✗`) now, all one cell, so a terminal without an emoji font draws the line. The
  module `Icon` fields in `trainer/types.go` still held emoji at this slice; no
  renderer draws them (the menu has drawn the status glyph instead since the
  status-word slice), and they are domain data, so the clone stayed untouched here.
  They are Nerd Font glyphs now (L31). `TestNoInstallerScreenDrawsAnEmojiBox`
  no longer excludes the trainer, so the class rule now covers all 96 screens.

Guards (`trainer_cards_test.go`): the six screens draw the grammar's borders, rule
and chip at 160x50 and none of them at 80x24; every screen keeps its own words; the
menu card is the profile it read (states, meters, ladder, accuracy, weakest) and its
top border does not move as the cursor walks it at a short wide frame; the playing
screens keep the code gutter, the mission, the answer field and the caption that
names the window; the result card keeps the verdict, the facts, the explanation and
both buffers; the fight's card states the run's numbers, the verdict and the
measured time and best; all six fit 160x50/140x44/200x60/227x62/160x24; and no frame
of any screen at either size draws a box emoji with any message `update.go` can
assign to `TrainerMessage` (the guard parses the file, so it measures the handler's
copy rather than a second copy written by hand). Goldens: `TestTrainerMenuWideGolden`,
`TestTrainerLessonWideGolden`, `TestTrainerBossWideGolden`,
`TestTrainerResultWideGolden` and `TestTrainerBossResultWideGolden` are new;
`TestTrainerMenuGolden` and `TestTrainerBossGolden` were regenerated for the emoji
removal (the changed lines are exactly the `⚠️ Weakest:` prefix and the `⚔️ ` on the
boss header). The review capture now writes the trainer's six screens too.

Not taken: the compact bodies still draw chips and guttered blocks rather than cards
— that is what the floor measures, and the cards need two columns. The result and the
boss-result screens have no two-column shape (they are one card across the
composition), so their 80x24 layout and their 160x50 card are the same shape at two
measures.

L31. The three issues this branch still carried, all of them older than the last
slices and none of them inside one. Each is closed with the guard that already
measured it, not with a guard that was relaxed.

- **The still frame reserves the creature's block.** `frameWithPanels` asked
  `companionSprite` for the rows to reserve, and `companionSprite` answers nothing
  while `Animating` is false -- correct for a renderer, wrong for a reservation. So
  the still frame centred the body in every row between the rules and the animated
  one centred it in the rows above the creature: turning the gate on moved
  `MainMenu` and the five pickers by half the block (two rows at 80x24), which is
  the one thing a reserved block exists to prevent. The reserve is now measured by
  `companionBlockRows`, which reads the rung the terminal selected and the art the
  state draws and ignores the gate: it probes a copy of the model with `Animating`
  set, so the frame on screen keeps its clock, position and gaze. A screen where the
  block genuinely cannot be drawn -- a stage too narrow, art that overflows its rung
  -- still reserves nothing, in both frames. The guard is the one that failed:
  `TestScreensRenderIdenticallyWithAnimationOnAndOffExceptTheCompanionRow`, unchanged
  in what it demands (the frame neither gains nor loses a row, and no body row may
  differ), now green at all three measured sizes for all nineteen screens. Its
  sibling `TestCompanionTicksChangeOnlyItsOwnRows` and the whole `Anim` set stay
  green. The still frame of every framed screen that reserves moved up by half the
  block, which is the fix: goldens regenerated are the seven wizard pickers and the
  main menu in both widths, plus `TestMainMenuTruecolorGolden`
  (`TestMainMenuGoldenIsIndependentOfTheWorkingDirectory` reads the same file).
- **The Methodology card carries the method and the attribution.** The wide shell
  audit's card held five generic bullets -- command, median rule, keys, promise --
  and the attribution, the plan count and the run list existed only in the compact
  body, so at 200x60 `TestScreenShellAuditShowsTheMethodTheMedianAndTheAttribution`
  could not find them. The cut was not the card's budget losing content: the card was
  drawn whole, eight rows, with ten rows of the frame unused, and the prose was
  never in it. The card now draws the compact body's own paragraphs for the method
  (the plan, in the words of `shellAuditPlanParagraph`), its attribution (the
  instrument's rows, the declared count of rows it did not name, what the numbers
  mean), the run list under `shellAuditRunListLead` and the read-only promise
  (`shellAuditReadOnlyPromise`); the recommendations still come first when the user
  opened them. The compact body is byte-identical -- its paragraphs became those same
  two functions -- so `TestShellAuditGolden` did not move. The card is budgeted by
  the rows the bands above it left (`cardChromeRows` off the budget) and a card
  taller than that ends with the description's own declared cut, `… and N more`,
  rather than being sliced by the frame's `body[:budget]`. At 200x60 the card fits
  whole; at 160x50 it declares nine lines it kept back. The attribution is drawn in
  blocks (`shellAuditAttributionTable`, `shellAuditAttributionCut`,
  `shellAuditAttributionClosing`) so the declared cut starts its own line: a clause a
  wrap splits in half is a cut a reader can miss, and the paragraph's words are
  unchanged. Guards: the paragraph guard above, plus the new
  `TestShellAuditMethodologyDeclaresTheLinesItCouldNotDraw` (the same state either
  fits whole, with its attribution and its promise, or declares a positive count, and
  neither frame is taller than its terminal).
  `TestShellAuditWideGolden` was regenerated for the card's new rows.
- **The module icons left the emoji vocabulary.** `ModuleInfo.Icon` is the module's
  identity, not a row -- the trainer's menu has drawn the status glyph since the
  status-word slice -- but it still held emoji, which a terminal without an emoji
  font draws as a box. The field, its nine assignments and every other datum stay;
  the glyph is the interface's own now, one Nerd Font cell each, commented at the
  assignment: `nf-fa-arrows-h`/`-v` for the two motion modules, `nf-fa-crop` for text
  objects, `nf-fa-repeat` for change & repeat, `nf-fa-exchange` for substitution,
  `nf-fa-search` for regex, `nf-fa-cogs` for macros, `nf-fa-undo` for editing and
  `nf-fa-clipboard` for registers. `TestGetAllModules_HasRequiredFields` now holds
  every icon to the private use area, so a return to emoji fails there even though no
  renderer draws the field; `TestGetAllModules_HorizontalIsFirst` pins the first
  module's glyph.

Validation was the short loop the three issues name: the three guards, the golden
set, the `Anim`, `Companion` and shell-audit sets. No full suite, no preflight.

L30. The informational findings of the closed reviews were triaged: T6 (utilities,
`review-a936b05a007a4b9d`), T7 (theme selector, `review-0ce2625867193a4f`), T9
(remaining pages, `review-a1147e36ffc35a3a`) and T4 (dashboard, escalated and left
unvalidated by the user). Every finding was non-blocking and none reopens a review.
The pass changed comments, test clarity and one separator; no behaviour, no key and
no datum moved. The three T6/T7/T9 lineages had already been burned to their
termination records, so each finding was read at the line it names in the worktree
that raised it (`/tmp/review-t6|t7|t9`) and in the tree it left behind.

- **The utilities summary's separator lost a space.** `utilitiesOverviewCardTitle`
  joined the panel's name, a space and `panelTabSeparator` -- which is itself
  `" · "` -- so the collapsed panel's title read `System Overview  · Linux`. It joins
  with the separator alone now, the same shape the panel layer's own title uses.
  `TestUtilitiesGolden` was regenerated for the one line that moved; the wide
  golden, which draws the panel in its own column, did not move.
- **The floor guard's OS fallback no longer forgives a missing shell.**
  `TestUtilitiesFloorKeepsTheCardAndTheFacts` asked for the OS and the shell in one
  loop with `|| strings.Contains(plain, "Linux")` inside it, so a frame that dropped
  `zsh` passed on the word `Linux` alone. The OS is asked once (a distribution's own
  name or `Linux`) and the shell separately.
- **The row guard reads the row, not the prose.** The section's rows are named in
  its notes and repeated in the panel's Selected fact, so a frame-wide
  `strings.Contains(plain, row.Title)` could pass with the row itself missing. The
  new `utilitiesRowDrawn` requires the name and the row's own gutter on one line --
  its icon, or the cursor's marker where the row under the cursor takes the icon's
  place -- and does not accept the panel's Selected fact, which repeats the row
  under the cursor behind the same marker.
- **`utilitiesRowWindow` documents its clamp.** An out-of-range cursor is read as the
  first row; the doc says so and says the clamp never writes back to the model.
- **The port row's comment promises only what the screen draws.** It said a platform
  with no reader "says so in the screen's own body"; the screen shows the reading's
  declared limits against the row they qualify (`portDetailLines`,
  `portFacts`), and a reading with no sockets says the list matched none. The
  comment at the row (`view.go`) and the one where the section offers it
  (`model.go`) now say that.
- **The update row's comment names the row it holds.** It claimed the row "names what
  the screen does rather than the feature"; the value is `Installed Tools`, the
  mockup's own name for the feature (L13), and the action is the row's subtitle,
  "Check latest versions". The comment now says so.
- **The theme picker's activity branch says what the slot costs.** The comment
  promised "the list keeps every theme"; `themeActivitySlotFloor` gives those rows up
  on a frame too short for the list and the slot, which its own doc already says. The
  branch comment now says the same thing.
- **`themePickerVisibleCards` documents what it counts.** It counts lines carrying a
  theme's mark, one theme to a line in the list the window guard reads it in; a grid
  draws two cards to a line, which the doc now says.
- **The picker footer guard names itself.** Its doc comment named
  `TestPickerQuestionsKeepTheirKeysAndTheirWayOut` while the function is
  `TestPickerQuestionsKeepTheirKeysAndTheirWays`; it names the function it guards and
  says what it holds (the keys, not the body).
- **The dead-end guard reads its own `want`.** The field was filled and never read,
  and the "topic with no page" case set `ViewingTool`, which the topic screen never
  reads: the case drew a real topic and passed. The case puts
  `SelectedLazyVimTopic` past the list, every case asserts its message, and the
  expected copy is the message the screen really draws (`Topic not found`).

Left as they are, each with its reason:

- T6 R2-006 and R3-001 (`TestUtilitiesWideGolden`'s torn row): the window-title
  escape and the teardown sequences are what `goldenTranscript` pins from teatest's
  raw stream, in 60 of the 81 snapshots. Stripping them is a harness-wide change, and
  hand-editing a snapshot is the one thing this pass may not do.
- T7 R3-001 and T9 R3-001: both are behavioural. The picker's activity slot is
  sized to zero rows when the rows left are `sectionMinEntryRows` or fewer, so the
  press's result is dropped while the notice has already given way;
  `referenceMaxScroll` returns `len(table.Rows)` when the budget cannot hold the last
  entry, an offset one past the end. Each wants a guard the shape and window
  arithmetic does not have.
- T9 R2-001 (`menuCardBody`'s `0` for the description cap): the parameter no longer
  exists. `renderSelection` calls `menuCardBody(title, description, hints)` and the
  description is uncapped by construction.
- T4 R2-001, R3-VHS-FLOOR-OUTPUT and R3-VHS-WIDE-OUTPUT: not applicable to this
  tree. `1b89e34` removed the GIF capture, so the tapes declare no `Output` by
  design; each step writes a PNG `Screenshot` and `scripts/screens.sh` owns the
  path. `vhs/` is also outside this pass's surfaces.

Validation was the short loop: `gofmt -l`, `go vet ./internal/tui`, the utilities and
theme-picker sets, the picker and dead-end guards, the learn-table window guard, the
port manager and update center sets and the goldens -- 140 focused tests green,
`TestUtilitiesGolden` regenerated by the fix it names. No full suite, no preflight.
The alternate-theme VHS capture stays out of this pass: the seeded HOME does not
detect the theme switch, so `Enter` opens Diagnose WSL instead of the alternate
theme, and the capture stays pending.

L32. The full suite `go test ./internal/tui/ -count=1` surfaced seven red guards (13
subtests): the six `…FallsBackToTheCompactListWhenTheCardCannotFit` questions at
36x24 and 80x12, and `TestChoosingAThemeIsOneChangeNotTwo`. The first hypothesis on
the report -- shared mutable state leaking between tests -- was wrong, and the
reproduction says so: each of the seven failed the same way when run alone
(`-run` per name), so nothing was leaking and no guard was weakened.

- **The pickers' compact floor had become the menu card.** `renderSelection` fell
  back to `menuCardBody`, which since `b1d2b8f` draws the page's own name as the
  chip on the card's border. At 36 columns the gate (`questionCardWidth`, unchanged
  and correct) refused the card and the fallback then drew a card one row shorter
  that still wore the title: the guards read the chip as "the card is drawn", and at
  80x12 the same fallback drew the question past the frame's rows. The fallback is
  now `selectionCompactBody`: the page's title and note as rows, then the shared
  card's bare box (no chip) holding the same options one line each, windowed around
  the cursor with the note yielding first. `selectionAsksQuestion` names the six
  once, so the card's dispatcher and the fallback's dispatcher cannot disagree, and
  the terminal warning -- the one renderSelection screen that never had a question
  card -- keeps the menu body it had. The gates were not widened; a 36-column frame
  now falls back in every order.
- **The theme picker's activity slot was not padded.** `themePickerBodyLines` clipped
  the activity to `shape.Activity` rows but did not fill them, so the pending frame's
  body was four rows shorter than the result's and the frame's vertical centring put
  the whole list two rows higher than the frame the result landed on. The slot is
  now padded to the height it reserved (`padRows`), so the pending line keeps the
  slot's first row, the list keeps its row, and the press reads as one change.
  `TestThemePickerActivityGolden` was regenerated by that fix; its row count is
  unchanged and its pending line still sits under the list in the result's rows.

Validation: the seven guards (13 subtests), the theme-picker set and the golden --
35 tests green -- and the full package suite `go test ./internal/tui/ -count=1`, ok in
463s with no failures. `gofmt -l` clean, `go vet ./internal/tui` clean. No other
package, no preflight, no commit.

L33. The update center's run has a live progress surface, in the tree and
uncommitted. L16 and L19 recorded the update center as the section's remaining
follow-up; this is its running half, added on top of the table, the detail card and
the keys the screen already had (T11 in the table above).

**One set of facts, drawn twice** (`update_center_progress.go`). The surface is
built from the model's own run state and nothing else:

- `updateCenterRunSteps` is the sequence as it runs: every command whose
  `ApplyResult` came back, then -- while `Applying` -- the one in flight and the
  ones behind it. The finished half is read from `Applied`, not from `Plans`,
  because the re-read that follows a run replaces the plans with the managers'
  fresh answers while the run's report has to outlive them.
- `updateCenterRunTally` counts the entries: `done` is the outcomes the finished
  commands answered for, `total` is the entries the plans carry, `failed` is
  `ApplyResult.Failed()`. The bar is `meterRendered(done, total, cells)`, so its
  fill is whole entries of that fraction -- no timer, no percentage the managers
  did not report.
- `updateCenterRunOutcomeCounts` splits one result into confirmed / executed /
  refused / unknown, so a row says "1 confirmed, 1 not moved" and never folds a
  stopped run's unknown into a refusal.
- `updateCenterRunFailures` is every entry a finished command did not move, with
  the engine's own `Detail`; the manager's reason for the command is what it
  carries. Entries a stop never reached stay in `NotReached` and are drawn apart,
  as the report paragraph `- <name> — the run was stopped before its command ran`.
- `updateCenterRunLogRows`/`updateCenterRunLogParagraphs` are `updates.Tail` of the
  model's bounded `Log`, behind the same `> ` prompt the installing screen's log
  card draws.

**The wide surface** is `updateCenterRunCard`, one `cardGroups` in the reference
grammar: the bar and its count, `Commands` (one row per command, windowed around
the one in flight, each led by ✓/✗/▸/○), `Not moved` (the failures with their
reasons, cut with the frame's marker, padded to two rows), and `Output` (the last
three lines the manager printed, padded). The caption says `step 2 of 3` while it
runs and `finished` after. It takes the right column above the Actions card while
a run is on the screen; the entry card the cursor described comes back when the
report is dismissed by the next run or replaced by the confirmation.
`updateCenterDetailColumn` drops the card whole if the column cannot hold it beside
the keys, and the height is a function of the sequence and the reservations only,
so a result landing or a line arriving rewrites rows that already exist.

**The floor** keeps the section renderer and gets `updateCenterRunParagraphs`:
exactly four lines, because the update center's detail band is four rows on every
frame and the fifth line of a longer list would be the frame's cut marker --
bar and count, `Step 2 of 3 · snap (system)`, `Moving core22`, then the newest fact
(a failure, else the last command's score, else the end of the output). The old
running prose is gone: the sentence about an update running and Escape not leaving
spent the band on what the status bar, the stop row and the footer already say.
The settled state leads its prose with `updateCenterRunReportParagraphs` (the
failures and their reasons), which is what keeps a failure readable after the run
at the floor; the wide shape keeps it in the card.

**Two facts the wide frame had nowhere to put** now have a row: the footer's middle
line is `updateCenterRunFooterLine` (`Running 2 of 3 · snap (system)`) while a run
is in flight and `updateCenterRunResultLine` (`Finished: 2 confirmed, 2 not moved`)
after it, and the frame's state word is `Installing…` while the sequence runs --
the same five words S2 fixed. The settled `Ready` and every row of the table, the
detail card and the keys are unchanged.

Guards (`update_center_progress_test.go`, 7 tests): the tally's four cases and the
fixture's own arithmetic; the wide surface's facts and that its bar fills
`done/total` of its cells (10 of 20 for 2 of 4) rather than a figure the screen
rounded; composition stability across a landing result and six arriving lines (the
frame's row count, the composition's span and each card's span are identical
before and after, while the words change); the floor's four rows and the absent
old prose; the failure named with its reason after the run in both shapes; the
boundary (no run, no surface; a confirmation hides the last run's report); the
checklist's order, glyphs and window; and the keys, which start nothing while the
sequence runs. `TestUpdateCenterRunningGolden` (new, 160x50) pins the wide surface;
`TestUpdateCenterWideGolden` is byte-identical, because the settled screen did not
move.

Validation was the short loop: `gofmt -l` clean, `go vet ./internal/tui` clean, the
update center's 48 tests, the frame/section-wide/capture guards (1195), and every
golden (86, the new one included). No full suite, no preflight, no commit.

L34. T12 landed in the tree (uncommitted): the installer's detail is rows, not
paragraphs. The user's two reports -- the "Diagnose WSL" wall of text and then the
"Development caches" screen -- asked for the same rule: nowhere in the installer
should there be a lot of text, and where a datum cannot become a row it keeps one
short sentence with the reason in the code.

**The mechanism.** `sectionScreenAreas` (facts_cards.go) is the registry: a screen's
detail is `[]factArea` -- the name the card's divider carries and the rows in it --
and `sectionScreenFacts` is its flatten, so the compact body's prose (factProse) and
the wide card (factLines, one group per area) read one list. `sectionWideDetailCard`
draws the groups through `cardGroups`, so a long detail is a set of short sections
rather than one column of facts; the card's height is still the tallest reading any
cursor position draws, measured in rows *and dividers* (`cardGroupRows`), so the
composition does not move as the cursor walks.

**The WSL advisor.** Its environment is now five areas of short rows -- Instance
(runtime, distro, kernel, processors, memory, swap), Storage (distribution, project
disk, drives, mounts), Resolver, Windows (.wslconfig and its three keys, an absent
key named as such), Elsewhere (the shell-startup pointer) -- plus This report
(read-only, what the copy writes), Not read (one row per reading that did not
happen) and Finding. The finding's title, severity, origin and scope were one value
("… — Notice (documented behaviour; changed inside this distribution, and applied
only after WSL restarts it).") and are four rows; its evidence, why, benefit and
what-to-do are one labelled row each.

**The disk cleaner, first half.** `diskCleanerAreas` splits its detail into Under the
cursor, Preview, Measured, Marked, The scope and What it will not do: the sizes (on
disk, contents, files, source), the reason each finding was found, its paths, its
reconstruction cost, the totals and their caveats, the selection split by risk, the
project directories, and the engine/WSL/privacy promises -- each one row.

**The disk cleaner, second half: the list is the grammar's card menu.** The headings
became the card's dividers, each finding a two-line row (a one-cell state mark in the
gutter, the name on the title line, the size and what removing it costs on the muted
line), the cursor's row filled; the row under the cursor is drawn by the same
`cardRowLines` as every other list. The presses are the footer's: `[s]`, `[p]`,
`[d]`, `[f]`, `[c]`, Space on a finding, and Escape. `diskCleanRows` is the list
(headings and findings) and `diskCleanerActions` is the press list the handler and the
footer both read, so a key cannot be advertised on a state that does not accept it;
the cursor no longer rests on a heading, and `diskCleanFinding` falls back to the
first finding so the card never goes blank on one.

**The rest of the audit.** The doctor and its confirmation, the network diagnosis
(every state, with the saved report kept as one labelled block because it is a file's
own lines), and the WSL resources, benchmark and migration screens are areas of short
rows too. The benchmark now gives each operation its own row under its target's
divider, and the migration preview gives each warning and each by-hand command its
own.

**What is still prose, and why.** The advisor's finding argument (`Why it matters`,
`Benefit`, `What to do`) is the model's own measured claim and stays full length: it
is one labelled row each, but cutting it to fit a row would either drop a fact or
invent a shorter one. `factArgumentLabels` names those three labels in the guard so
the exception cannot spread. The shell startup's explanation, the old update screen
and the theme picker still write sentences: their prose is the methodology and the
plan, not a set of values, and they are the next slice.

**Floor and arithmetic.** `diskCleanCardFloor` is 10 (it was 7) because a row costs
more lines than a sentence did and the evidence may not be cut; the list windows and
says how many findings it hid, and the header's range counts findings, not headings
(`diskCleanerRangeVital`). Raising the floor exposed a real bug in the shared band
arithmetic: a detail that only just fitted lost the row the clamp had already taken,
so `sectionBandsFor` now gives up the gap and the detail together when they do not
fit the available rows.

Guards: `TestEveryConvertedRowIsALabelAndAShortValue` (a row carries its label, its
value fits two lines of the value column, and only a finding's argument is exempt),
`TestTheDetailCardDividesTheConvertedScreensByArea` (the advisor's dividers are
named), `TestDiskCleanerListIsACardMenuAndItsPressesAreTheFooters` (the headings, the
two-line finding rows, no press drawn as a row, every press in the footer),
`TestDiskCleanerFooterCarriesEveryPressAndTheListCarriesNone` (each press is offered,
named and acts; Enter runs nothing; Space is only a finding's switch) and
`TestDiskCleanerListCarriesTheHeadingAndTheFinding`. Goldens updated:
TestWSLAdvisorGolden, TestWSLResourcesGolden, TestDoctorGolden, TestDoctorWideGolden,
TestDoctorFixGolden, TestNetworkDiagnosticsWideGolden, TestDiskCleanerGolden,
TestDiskCleanerWideGolden; new: TestWSLAdvisorWideGolden, TestDiskCleanerPreviewWideGolden.

Validation was the short loop: `gofmt -l` and `go vet ./internal/tui` clean, the
focused tests (WSL, SSH, doctor, network, disk cleaner, section-wide, frame,
utilities, goldens) and then the whole `internal/tui` package green (504 s). No
preflight, no captures, no commit.

L35. Two defects the user reported against the home screen, in the tree and
uncommitted: Tab on the welcome screen did nothing, and the wide composition's
margins and separation were unbalanced.

**Tab.** The welcome screen's body says "or explore the menu with [Tab]" and its
footer says "[Tab] menu", but the press did nothing at every size where the panel
is not drawn. The key path, not the copy, was wrong: the frame's panel walk
(`handleKeyPress`, `update.go`) runs BEFORE the per-screen dispatch and takes Tab
whenever the screen offers more than one panel. The welcome screen offers three
(the machine, live and tip panels), so the walk consumed the press and wrote
`PanelIndex` -- which the wide home never draws, because its right column is the
reference's three cards -- and the screen's own `case "s", "tab"` never ran. L19
shipped "[Tab] opens the menu"; the panel walk added later re-stole the key.

The promise is the truth, as the reference's footer says it: the panel walk now
skips the welcome screen (`key == "tab" && m.Screen != ScreenWelcome`), so the
screen's own dispatch reads the press and opens the main menu at every size. The
floor had been advertising the other meaning -- `panelHints` appended "[Tab] panel"
to the welcome screen's hints -- so it now keeps the screen's own "[Tab] menu" and
appends nothing: a footer may not advertise two meanings for one press
(`hintsNameTab`). `welcomeHints` is the screen's one footer at both shapes. Every
other key on the screen is untouched, and every other screen keeps the panel walk
(the trainer still reads Tab as "hint" because it offers no panel).

**The composition.** `home_view.go` drew the wide home as two columns of blocks
that were individually centred (`CenterHorizontally` per line) beside full-width
cards, both columns floating in the frame's middle: at 200x55 the left column ran
28 rows against the right column's 18, with eight blank rows above the lockup and
another band under the cards, and the gap between blocks was whatever that block's
blank line happened to be. The composition is now one set of measures:

- `homeTopMargin` (1 row) is the blank rows BOTH columns open with, so the two
  columns start on one row and neither block sits against the header's rule.
- `homeBlockGap` (1 row) is the gutter between two blocks of either column. The
  columns' own gutter is `layoutGutter`, which `composeColumns` already spends on
  every two-column screen; the composition adds no third measure.
- Every block of the left column is drawn against the column's own left edge, the
  lockup included (`homeLeftBlocks`). The tagline, the chips, the greeting, the
  environment sentence and the call to action now share one left edge; the screen
  used to centre each line of text over a flush-left band of cards.
- The chips are the dashboard's own four cards (`dashboardCardsRows`), so the wide
  home and the 80x24 floor draw ONE card shape and the wide screen's chips lost
  the fourth row the private `homeMachineCards`/`machineCardOf` pair spent.
- The composition is anchored, not centred (`placeHomeBody`): the rows the columns
  do not spend stay under them, above the creature's `companionBlockRows` reserve,
  which is taken off the budget before either column is measured. A block that does
  not fit is left out whole and the gutter is given back first (`stackHomeBlocks`),
  so a tight frame loses spacing before it loses a fact.

Both target sizes read deliberately from the same measures: at 200x55 and 160x50
every inter-block run is one row, both columns start one row under the rule, and
the chips sit in the tagline's column. The columns are still not the same height --
the left one holds the lockup, the tagline, the chips, the greeting and the call to
action, while the right one holds three cards and only the model's own facts fill
it (a machine card with no detected host, or a run with no backup, is a shorter
column). Stretching the right column's gutter to force equal bottoms would have
turned two cards into islands, which is not a rhythm; the leftover rows are the
creature's band instead.

Guards (`home_view_test.go`, 6 tests): `TestWelcomeTabOpensTheMenuAndNeverWalksPanels`
(200x55, 160x50, 160x40 and the 80x24 floor: the screen offers [Tab] menu, the press
opens the main menu with cursor 0, and `PanelIndex` is still 0),
`TestWelcomeStartAndQuitStillDoWhatTheyDid` (the fix moved no other key),
`TestHomeWideSharesOneTopMarginForBothColumns`, `TestHomeWideSeparatesEveryBlockByOneGutter`
(every inter-block run is `homeBlockGap` in both columns, and the columns are
`layoutGutter` apart), `TestHomeWideChipsSitOnTheTextsLeftEdge`, and
`TestHomeWideFitsItsFrameAndKeepsThePet` (exactly the terminal's rows, no row past
the width, no cut marker, the creature's block still drawn). Goldens: new
`TestHomeWideGolden` (200x55) and `TestHomeStandardGolden` (160x50);
`TestDashboardWideGolden` and `TestWelcomeLivePanelGolden` (160x40) regenerated for
the composition, and `TestWelcomeScreenGolden` (80x24) for the floor's footer, whose
only change is `[Tab] panel` -> `[Tab] menu`.

Validation was the short loop: `gofmt -l` and `go vet ./internal/tui` clean, the
home/welcome/dashboard/panel/frame/layout golden set, and the whole `internal/tui`
package green. No preflight, no captures, no commit.

L36. T12's remainder landed in the tree (uncommitted): the last three screens that
still wrote paragraphs -- the shell startup's detail, the old update screen and the
theme picker's plan and refresh review -- are rows now. L34 named them as the next
slice; this is that slice, and it closes the "nowhere in the installer is a wall of
text" rule for the screens it named.

**The shell startup.** `shellAuditDescription` is now `factProse` of
`shellAuditAreas()`, and the wide frame's Methodology card is `cardGroups` of the
same areas, so the floor's band and the card read one list. The areas are The
method, The plan, What the runs support, The measurement, The attribution, The clean
comparison, The trend, the Recommendations, The starts and What it will not do. What
the old paragraphs said, row by row:

- The method paragraph (`shellAuditPlanParagraph`) is six rows: the press's exact
  line, the bound every start gets, "the median: the middle start, not an average
  and not a single run", what that number describes, the one measurement that
  separates the shell's startup from the user's configuration, and the directory of
  the utility's own that the clean start is pointed at.
- The plan-and-keys paragraph is five settings rows (`[m]`/`[n]`/`[w]`/`[b]` are
  the row that names what they change, so no key is lost) plus the history's two
  states: the path it is stored in and what the file holds, or the row that says
  nothing is written and `[h]` stores the runs.
- The statistics paragraph is five rows: how many finished starts are behind them,
  the mean, the spread with its reading, the 95th percentile -- or, from a count
  that cannot support one, a row labelled "No percentile" that says how many runs it
  needs, which is the same reason that number was never taken -- and the trimmed
  median with its fence and the count of starts left out. `shellAuditStatisticFacts`
  replaced `shellAuditStatisticsParagraph`; the guard that held the sentence to one
  count per number now holds the rows to it.
- The context paragraph is the measurement's own rows: shell, version, the start,
  the command, the terminal (or that none was attached) and, when the model has one,
  the note about what the start still includes.
- The attribution paragraph is one row for the instrument (which one ran, how many
  entries it reported, and how its order was decided, in that instrument's own
  words), one row per named entry in the instrument's own units (`#3 _omz_source --
  249.88 ms total, 244.60 ms self, 22 call(s), 14.48% of the profiled start`), the
  declared cut as its own row, and the two claims the closing paragraph made as two
  rows: what a row means and that the profiled start is a start of its own.
  `shellAuditAttributionRow` returns the numbers now and the name is the row's other
  half, so a long function name is never truncated to the label column.
- The clean comparison is the two medians, the count each is made of, the difference
  with its share, and the reading the difference supports; the trend is before, now,
  the change, how many stored measurements it was read against, the oldest date and
  the reading. A comparison nobody asked for is not drawn, as before.
- The runs behind the median are one row (every completed start in the order it ran)
  with the row that says why they are listed, then one row per start that did not
  finish, so "left out of the median" and its reason are on the row with the count.
- The promise is three rows: what is written (nothing), that a slow function is not by
  itself a mistake, and where the fix belongs. The `shellAuditReadOnlyPromise`
  constant is gone because each clause is now a row; every word of it survives.

The wide Methodology card draws every area except the three the cards beside it
already hold (the action row's plan boxes, the Results card's numbers, the Recent
runs table), which is what keeps a row from being drawn twice on one frame. It is
still budgeted by the rows the bands above it left; a detail taller than that budget
now ends with ONE count of every row it kept back -- the rest of the area it stopped
in and all of the areas after it -- because `clipCardGroups` drops a whole area
silently and the last area is the promise: a promise that disappears without a word
is the one thing this card may not do. At 160x50 that card is five rows (the bands
above it are the reference's own), so it prints the method's first four rows and
`… and 30 more`; at 200x100 the whole detail fits and it prints every row. The
footnote is as small as the composition above it makes it -- giving that card more
rows is a composition call this slice did not take.

**The old update screen.** `updateDescription` is `factProse` of `updateAreas()`, and
`ScreenUpdate` joined the facts registry, so the screen's existing wide composition
draws the same rows in its detail card. The rows: what the screen reads and what it
does not touch; the state's own `displayRows` (this build, the channel, what is
published, when it was checked, why not, the last update); where the answer came
from, with the URL and the cache's own file and lifetime, or -- when nothing was read
-- the row that says the release is unknown here rather than current and the row that
names `[r]`; and, last, either the reason this copy is not the installer's to replace
(the one sentence `updateBlockedReason` words for the main-menu panel too) or the
three rows of installing it (what it would replace, what the download is checked
against, and the `.previous` copy that can be put back by hand).

**The theme picker.** The picker draws its own screen and has no detail card, so it
is not in the registry: `themePickerAreas()` feeds `themePickerDescription`, which is
`factProse` of it. The plan is five rows (defined once in `themes/`, choosing applies
it to every tool it can paint, it names the tools it leaves out, the cursor previews
without applying, the switch records what it replaces so it can be undone) -- the
paragraph that used to be the empty state's intro. The refresh review, which is the
picker's other prose state, is rows too: the count of managed files and that nothing
has been written yet, two rows per file under the file's own tool name (the path and
why it was found, then where the current file is preserved first), what Undo can put
back, and the two rows that answer it; then an area for the files that cannot be
refreshed. The result keeps its own lines as one labelled block, because a refresh's
outcome is the run's own rows and not a set of values the screen could split. The
palette board, the grid, the miniature previews and the tip are untouched: the tip is
already one row with a label and one short sentence, which is the rule this slice
applies, and the palette is still the definition's own colours.

**The documented prose exception.** Three rows per recommendation keep the model's own
sentences, and `factArgumentLabels` now says why: a finding's argument, the evidence
a recommendation rests on, the note a start still carries, and the reason a press, a
comparison or a file was refused are written by the engine and worded once, so
shortening one here would put a second wording of the same reason on screen. They are
still one label and one value each.

**Floor and arithmetic.** At 80x24 the shell startup's band prints four one-fact rows
and declares the rest (`… and 65 more`), where it used to print the first paragraph
and `… and 50 more`: the band is five rows either way, and what changed is that each
visible line is a labelled fact. The frame guards are unchanged in shape -- the shell
audit's twelve terminals, the theme picker's twelve, the update screen's twelve, the
composition's fit -- and the two devices that keep them honest still hold: the rows
`GetCurrentOptions` offers are on screen at every size, and no frame is taller or
wider than the terminal that asked for it.

Guards: `TestTheLastProseScreensDrawRowsAndNotParagraphs` (the four states -- shell
audit, update, the picker's plan and its review, and its result -- every row labelled,
every value a row and not a page, every value still in the compact body),
`TestTheShellAuditMethodologyCardDrawsTheRowsTheBodyPrints` (the wide card draws the
method, the context, the attribution and the promise as rows and does not repeat the
three areas the cards beside it hold). Rewritten to read rows instead of sentences:
`TestShellAuditAttributionNamesTheInstrumentThatRan`,
`TestShellAuditStatisticsRowsNameEachCountOnce`,
`TestScreenShellAuditShowsTheMethodTheMedianAndTheAttribution` (both shapes),
`TestShellAuditMethodologyDeclaresTheLinesItCouldNotDraw` (whole at 200x100, declared
at 160x50). Goldens updated: `TestShellAuditGolden` (80x24) and
`TestShellAuditWideGolden` (160x50); new `TestThemeRefreshReviewGolden` (160x45) pins
the review's rows. The theme picker's four goldens are byte-identical: the plan is
not part of its list state, and the grid, the palette and the tip did not move.

Validation was the short loop: `gofmt -l` and `go vet ./internal/tui` clean, the
shell-audit/update/theme/golden guards, the section-wide and capture guards, the
frame fit guards at every measured terminal, and every golden. No full suite, no
preflight, no captures, no commit.

L37. The dependency step's sudo credential, in the tree (uncommitted). The user
reported that installing the deps fails with "sudo password is required".

**The failing path, measured.** The dependency step is planned `Interactive`, so
an interactive run never reaches `stepInstallDeps`: it runs `getDepsScript`
through the captured executor, and the step loop asks for the credential through
`sudoPrimer`/`sudoTerminalPrimer` before the first step that needs root. The
step itself is what the **scripted** route executes -- `--non-interactive` ->
`runInstallSteps` -> `executeStep("deps")` -> `stepInstallDeps` -- and that route
never primed anything. Its first command is `sudo -n apt-get update` through
`runSudoWithLogs`, which hands the child pipes for stdin and stdout, so sudo has
no terminal to ask on and answers "a password is required"
(`sudoPasswordUnavailable` is the wrapper that turned it into "sudo could not ask
for a password in this run"). The order is `probe`, `ask`, then the commands:
the ask precedes the command it makes possible, and it is the same ask the run
makes.

**The wiring.** One seam, two forms of the handover, and the step uses it.
`sudoCredentialCached` is the first half of the run's request as a plain call
(`sudo -n -v`), shared with `defaultSudoPrimer` so "sudo already has what it
needs" is one question with one implementation; a machine that answers it is
never prompted. `sudoTerminalAsk` is `sudo -v` with the terminal attached, run in
place: the form a caller outside the program can use, where `sudoTerminalPrimer`
is the same command through `tea.ExecProcess` for a caller that owns the screen.
`system.RunTerminal` is the runner that attaches the process's own streams and
captures nothing, which is exactly what the capturing runners cannot do. The
step asks only when its **plan** needs root (`platformInstallPlan.needsRoot`),
before the first command and never because of the step's name, and it asks at
most once per run: a run that already made its request -- cached or refused -- is
left alone, and a refused ask does not abort the step, which still reports the
missing credential through `sudo -n` with the commands to run by hand. Nothing
is stored and no password passes through the program.

Guards: `TestTheDependencyStepAsksForTheCredentialBeforeItsFirstRootCommand` (the
reported failure: exactly one ask, before the first root command, and the model
records it), `TestTheCachedCredentialIsNeverAskedFor` (a usable credential is
never prompted for), `TestTheDependencyStepDoesNotAskTwice` (the step loop's
request is not repeated), `TestTheDependencyStepAsksNothingUnderDryRun`,
`TestAUserScopedPlanAsksForNothing` (Homebrew and pkg ask for nothing),
`TestARefusedCredentialDoesNotAbortTheDependencyStep`,
`TestTheDependencyStepAsksTheSameWayTheRunDoes` (a fake `sudo` on PATH: the probe
is given `-n -v` and no `-S`, the ask exactly `-v`),
`TestTheTerminalAskFailsWhereThereIsNoTerminal`, `TestTheRootPlanIsTheOneThatAsks`
(the plan's answer and `sudoCommand()` agree), and in `internal/system`
`TestRunTerminalGivesTheCommandTheTerminalAndCapturesNothing`,
`TestRunTerminalReportsTheCommandsFailure` and
`TestRunTerminalKeepsTheCallersDirectory`. `withDepsMocks` now stubs both seams so
no existing dependency test can reach real sudo.

No golden moved: the ask speaks only in the scripted route, and its line goes
through `SendLog`, which is verbose-only there. Validation was the short loop:
`make check` (gofmt, `go vet ./...`, and the changed packages
`./internal/system ./internal/tui`, every golden included) passed. No full
suite, no preflight, no commit.

L38. The update centre's apply asked for no sudo credential, in the tree
(uncommitted). The user reported that applying an update fails with "sudo
password is required".

**The failing path, measured.** `startUpdateCenterApply` opened the output channel
and returned the first plan's command; `Engine.Apply` is built from
`Plan.Command`, whose system-wide plans are spelled `Elevate: true` and therefore
`sudo -n -- <manager> upgrade …` through `Executor`-free `runCommand`, which hands
the child pipes for both streams. `-n` is the flag that refuses instead of asking,
so a machine whose sudo would want a password answered "a password is required"
and the plan was classified `StatusPermission` -- the same failure the dependency
step had in L37, on the path that reaches a root command without an installation
run behind it. The update centre is openable from Utilities on its own, so it could
not rely on the step loop's request: nothing had ever asked.

**The wiring.** The apply makes the run's own request, before its first root
command, and the screen still owns no prompt of its own:

- `updateCenterApplyNeedsRoot` reads the plans' own answer (`Plan.Elevation`, the
  same one the confirmation names), so the request follows the command and not the
  screen's name: a Homebrew-only sequence asks for nothing.
- `sudoRequestPending` is the run's "at most once" rule, shared with
  `shouldPrimeSudo` so the step loop and this screen cannot disagree: a run that
  already answered the question -- cached, typed or refused -- is never asked
  again.
- `sudoCredentialCheckCmd` asks `sudo -n -v` off the update loop (through
  `sudoCredentialCached`, unchanged), so a machine whose sudo already holds a
  credential -- a NOPASSWD rule, a container running as root -- is never prompted.
- Only where it does not, `sudoTerminalExec` hands the terminal to `sudo -v` and
  waits: the program's form of the ask, spelled once and now shared with
  `defaultSudoTerminalPrimer`, so the installing screen resumes its step loop on
  `sudoPrimedMsg` and the update centre resumes its own command sequence on
  `updateCenterCredentialMsg`. The scripted route keeps `sudoTerminalAsk` and
  `system.RunTerminal`, where there is no screen to release.
- `updateCenterBeginApply` starts plan 0 and its output reader; a refused ask starts
  the sequence too, because the command that needed the credential reports the
  refusal itself through `sudo -n`, in the manager's own words. Nothing is stored,
  no password travels through the program, and no `-S` is involved.

The screen's progress surface, plan, keys and data are unchanged: `Applying`,
`Lines`, `PlanIndex` and the stop key are set exactly as before, and the only new
state is the run's own `SudoPrimeAttempted`/`SudoPrimed`. The dry run is untouched
(the update centre never consulted it, before or now).

Guards (`update_center_credential_test.go`, 8 tests, every seam a variable):
`TestTheUpdateApplyAsksForTheCredentialBeforeItsFirstRootCommand` (the reported
failure: one probe, one ask, both before the first `run:apt`),
`TestTheCachedCredentialIsNeverAskedForBeforeTheUpdateRuns`,
`TestTheUpdateApplyAsksForTheCredentialOnlyOnce`,
`TestAUserScopedUpdatePlanAsksForNothing`,
`TestAnUpdateApplyAfterTheRunsRequestAsksNothing` (both the cached and the refused
case), `TestARefusedCredentialDoesNotSkipTheUpdateRun`,
`TestTheUpdateApplyAndTheInstallingScreenAskThroughTheSameHandover` (one handover,
two answers) and `TestTheUpdateApplyAsksWithTheRunsOwnFlags` (a fake `sudo` on
PATH: the probe is given `-n -v` and no `-S`, the ask exactly `-v`).

No golden moved: the ask speaks before the run starts and writes nothing to the
screen. Validation was the short loop: `make check` (gofmt, `go vet ./...`, and
`./internal/tui`, every golden included) passed. No full suite, no preflight, no
commit.

L39. The wide home's composition was anchored to the header's rule and left the
frame's surplus as one band of void under the cards, in the tree (uncommitted).
The user reported it against the welcome screen.

**What was there.** L35 fixed the horizontal grammar and, in the same pass, anchored
the vertical one: `placeHomeBody` filled the body rows under the composed columns
and left the rest under them, on the reasoning that centring had produced two bands
of space instead of one. One band is what the user saw: at 200x55 the block spent
34 of the 45 rows above the creature and the other 11 sat as a single void between
the call to action and the pet.

**The fix.** `renderHomeWide` now reserves the creature's block first
(`companionBlockRows`, unchanged, so the reserve still ignores the animation gate)
and centres the block in the rows that are left between the header's rule and that
block; `placeHomeBody` places it with the same split every framed screen uses (the
odd row, if any, below). The tail under the block is the creature's band, so the
pet stays the frame's last rows. The rhythm is untouched: `homeTopMargin` is still
the block's own first row (both columns still start together and the block never
touches the rule), `homeBlockGap` is still the one gutter between blocks,
`layoutGutter` is still the columns' gutter, and every block is still drawn against
its column's left edge, chips included. Nothing about the block's height depends on
a clock, a tick or the cursor, so its row is a function of the terminal and of the
data the model already holds, and the 80x24 floor keeps the compact dashboard
unchanged (its goldens are byte-identical).

Guards (`home_view_test.go`): `TestHomeWideCentresOneBlockBetweenTheHeaderAndThePet`
replaces `TestHomeWideSharesOneTopMarginForBothColumns` -- both columns start on one
row, at least `homeTopMargin` rows under the rule, and the rows above the block and
the rows between it and the creature's block differ by at most one, at 200x55,
160x50 and 160x40. New `TestHomeWideLetsNothingMoveTheBlock` renders the same model
before and after an animation tick and reads both columns' first row back: the
block stands still while nothing on screen changes. The rest of L35's guards are
unchanged and still pass (one gutter between blocks, `layoutGutter` between the
columns, the chips on the tagline's edge, the frame's rows and the creature's
block).

Goldens regenerated, blank rows only: `TestHomeWideGolden` (200x55, the block five
rows lower), `TestHomeStandardGolden` and `TestWelcomeLivePanelGolden` and
`TestDashboardWideGolden` (160x40/160x50, three rows lower). No drawn row, value or
word changed in any of them, and `TestWelcomeScreenGolden` (80x24) is untouched.

Validation was the short loop: `make check` (gofmt, `go vet ./...`, and
`./internal/tui`, every golden included) passed. No full suite, no preflight, no
captures, no commit.
