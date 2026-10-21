# Installer redesign — the mockup's design language

The user supplied a six-panel mockup (Dashboard, Main Menu, Setup, Installing,
Utilities, Theme Selector) and asked for a large interface refactor in its style,
including every page the mockup does not show. The image lives outside the repo
and is not committed; the specs below record what it shows and the exact strings.

## Specs

S1. **The design language.** The mockup's layout is the target: bordered chrome,
    two-line rows with Nerd Font icons, a filled selection bar, cards, the setup
    table, the theme grid and the progress screen. The palette is **not** copied
    literally: every surface draws its roles from the active theme (the themes the
    installer already ships), so the violet is the mockup's illustration and not a
    new required theme. Every colour still clears the existing contrast floors
    (body 4.5:1, the muted role on the drift, the key tone).
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
| T1 | S1 | The palette adaptation: the new surfaces (bars, cards, table, grid) draw from each existing theme's roles; no new theme; the theme grid's Default card is the current default theme | (pending) |
| T2 | S2 | The frame chrome: the header bar and the status bar, the per-screen state words, the footer hints at the right | (pending) |
| T3 | S3, S4 | The main menu: the two-line rows with icons and subtitles, the group titles, the selected row's detail panel and its button; the 80x24 floor keeps the one-line rows | (this slice, current palette) |
| T4 | S5 | The dashboard cards, the greeting with the user's name, the tagline and the call to action | (pending) |
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
