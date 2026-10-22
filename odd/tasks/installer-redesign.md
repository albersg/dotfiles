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
  terminal gave it and the 80x24 floor keeps its five-row shift.
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
