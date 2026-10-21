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
| T9 | S10 | The remaining pages | (in the tree, uncommitted — the pickers' card, the framed dead ends, the completion fact rows, the backup cards and the two-line learn tables; the doctor, the update center and the remaining utility pages are follow-ups, see L16) |
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
