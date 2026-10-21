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
| T1 | S1 | The theme pass: every new surface uses the applied theme's roles; the theme switch still recolours the installer and the system together; VHS captures under the default and one alternate theme prove it; no hardcoded palette | (pending) |
| T2 | S2 | The frame chrome: the header bar and the status bar, the per-screen state words, the footer hints at the right | (in the tree, uncommitted) |
| T3 | S3, S4 | The main menu: the two-line rows with icons and subtitles, the group titles, the selected row's detail panel and its button; the 80x24 floor keeps the one-line rows | (in the tree, uncommitted — layout and vocabulary in the current palette) |
| T4 | S5 | The dashboard cards, the greeting with the user's name, the tagline and the call to action | (in the tree, uncommitted — the dashboard, the cards and the named greeting) |
| T5 | S6 | The setup table and its right panel | (pending) |
| T6 | S8 | The utilities screen | (pending) |
| T7 | S9 | The theme selector grid | (pending) |
| T8 | S7 | The installing checklist, the log panel and the progress bar | (pending) |
| T9 | S10 | The remaining pages | (pending) |
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
