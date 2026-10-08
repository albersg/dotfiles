# Instalador — inventario de defectos de experiencia

**Alcance**: recorrido completo del instalador TUI (`installer/internal/tui`), read-only, sobre `origin/main`.
**Vara de medir**: el ejemplo del usuario (aplicar tema cambia la pantalla y ~1 s después cambia otra vez) queda fuera de alcance (lo arregla otro frente). Se usa sólo como patrón: *un cambio visible debe ocurrir en el momento de la pulsación, no cuando termina el trabajo en segundo plano*.
**Distinción**: sección A = **defectos** (algo que está mal, se arregla). Sección B = **preferencias** (algo que podría ser mejor, se decide). Sección C = lo que está bien. Sección D = hipótesis sin evidencia suficiente.

---

## A. Defectos (por impacto ÷ coste)

### 🔴 A1 — En la sección **Utilities**, `Esc` no hace nada, pero el pie anuncia `[Esc] back` · impacto ALTO · coste PEQUEÑO · **PRIMERO**

- **Qué ve el usuario**: entra en Utilities (tecla `u` o la fila). El pie dice `[Esc] back`. Pulsa `Esc` y **no pasa nada**; sólo sale la fila `← Back` (Enter) o `Backspace` (que no se anuncia).
- **Por qué es un problema**: la pantalla promete una tecla que no responde, en el hub desde el que se llega a todas las utilidades.
- **Evidencia**: `update.go:951` enruta *todo* `esc` a `handleEscape`; `handleEscape` (`update.go:1089` y siguientes) **no tiene caso `ScreenUtilities`**; el `case "esc", "backspace"` de `handleUtilitiesKeys` (`update.go:1443`) es código inalcanzable; `renderUtilities` declara `hintBack` = `[Esc] back` (`view.go:1178-1181`); la doc lo fija (`docs/tui-installer.md:545` → `Esc | Go back`).
- **Por qué se escapó**: el test `TestUtilitiesEscapeReturnsToTheMainMenu` (`update_test.go:1030-1037`) llama **directamente** `m.handleUtilitiesKeys("esc")`, saltándose el despacho real, y por eso pasa en verde mientras la tecla real está muerta.

### 🔴 A2 — Al abrir **Utilities** por primera vez, la sección afirma un hecho falso («no switchable here») y luego aparecen filas y cambia el texto · impacto ALTO · coste PEQUEÑO-MEDIO · **SEGUNDO**

- **Qué ve el usuario**: entra en Utilities. Ve *«The dotfiles' own theme is not switchable here: …»* y **no** ve la fila *Change the dotfiles theme* ni la de WSL. Un instante después aparecen esas filas (empujando `← Back` hacia abajo) y el texto se corrige.
- **Por qué es un problema**: durante ese hueco la pantalla **miente** (dice que la función no existe cuando se está leyendo), y el listado cambia bajo el cursor.
- **Evidencia**: al entrar se lanza la lectura asíncrona (`update.go:1141-1146`, `update.go:1397-1403` → `tea.Batch(dotfilesThemesCmdIfNeeded(), wslResourceStateCmdIfNeeded())`; `update.go:297-303`). Mientras no llega, `utilitiesDescription` cae en la rama «no switchable» porque `DotfilesThemes` está vacío y `DotfilesThemesErr` también (`view.go:1393-1398`), y `GetCurrentOptions` no ofrece la fila de temas (`model.go:554-591`). El resultado llega en `dotfilesThemesLoadedMsg` (`update.go:657-665`).
- **Contraste que demuestra que ya saben hacerlo bien**: WSL sí tiene un «*yet: reading...*» honesto (`view.go:1603-1605`) y el panel Live dice «*reading this machine…*» (`metrics.go:175-184`). El tema no tiene ese estado intermedio.
- **Resuelto** (`fix/no-lies-no-moves`): con `DotfilesThemes == nil` y `DotfilesThemesErr == ""`, `utilitiesDescription` dice *«Reading the dotfiles' theme definitions from the repository checkout…»* (predicado `dotfilesThemesPending`); nunca «not switchable» antes de una lectura terminada. Guard: `TestUtilitiesSaysItIsReadingTheThemeDefinitionsWhileTheReadRuns`.

### 🟠 A3 — Escribir el `.wslconfig` hace que la pantalla cambie **dos veces** (la primera, a «reading») · impacto MEDIO-ALTO · coste PEQUEÑO-MEDIO · **TERCERO**

- **Qué ve el usuario**: ajusta valores y pulsa `Write the .wslconfig`. Al terminar, el cuerpo **se reemplaza entero** por *«WSL resources are not adjustable here yet: reading the Windows profile and the host's capacities.»*; uno o varios segundos después vuelve la tabla con los valores releídos. Dos cambios visibles.
- **Evidencia**: pulsación → `wslResourceWriteCmd()` sin cambio de estado (`update.go:1551`, comando en `update.go:329-335`); al llegar `wslResourceWrittenMsg` se pone `m.WSLState.Resolved = false` y se relanza la lectura (`update.go:684-700`, línea `699`); `wslResourceDescription` consume `wslResourcesUnavailableReason` y con `!Resolved` devuelve el texto de «reading» (`view.go:1535`, `view.go:1603-1605`).
- **Resuelto**: el write ya no borra `Resolved`; marca `WSLState.Refreshing` y la relectura actualiza la tabla en su sitio. El slot del notice dice *«Refreshing the values from the file…»*. Guard: `TestWritingTheWSLConfigKeepsTheTableWhileItRereads`.

### 🟠 A4 — Aplicar un tema y «Refresh outdated theme files» no dan **ningún** feedback hasta que terminan · impacto MEDIO-ALTO · coste MEDIO

- **Qué ve el usuario**: pulsa *Apply the X theme*. No cambia nada. Tras escribir ficheros y ejecutar recargas (`bat cache --build`, `kitty @ load-config`, `tmux source-file`…), la pantalla **salta** de golpe a la vista de resultado; en ese salto desaparece el preview que teñía la UI. Igual con *Refresh outdated theme files*.
- **Evidencia**: `handleThemePickerKeys` devuelve `applyDotfilesThemeCmd(def)` sin tocar el modelo (`update.go:1665-1667`); el comando llama a `reloadThemeTools`, que **ejecuta comandos externos** (`installer.go:2096-2160`); el resultado sólo cambia el modelo en `dotfilesThemeChangedMsg` (`update.go:713-729`).

### 🟠 A5 — Las filas del menú principal se **insertan bajo el cursor** cuando llegan lecturas tardías · impacto MEDIO · coste MEDIO

- **Qué ve el usuario**: la fila *🔄 Restore from Backup* se **inserta en medio** (antes de Utilities/Exit) si aparecen backups; el panel gana una pestaña *Last install*; el plan gana *Overwrites*.
- **Por qué es un problema**: el usuario puede pulsar Enter esperando *Utilities* y obtener *Restore from Backup* (o *Exit*), porque la lista creció por debajo. «La interfaz bajo el cursor se mueve».
- **Evidencia**: `GetCurrentOptions` inserta Restore en medio (`model.go:514-526`, línea `523`) y `loadBackupsMsg` sólo asigna cuando llega (`update.go:648`); el panel *Last install* sólo aparece con registro (`panels.go:154-158`); *Overwrites* sólo con `len(m.ExistingConfigs) > 0` (`panels.go:805`), rellenado en `configsDetectedMsg` (`update.go:771-777`).
- **Resuelto**: `loadBackupsMsg` guarda la etiqueta bajo el cursor y la vuelve a encontrar (`selectedOption`/`holdCursorOn`), así Enter sigue abriendo la fila que estaba ahí. Guard: `TestALateBackupRowDoesNotMoveTheRowUnderTheCursor`.

### 🟠 A6 — `Esc` en la confirmación de restore salta al menú principal, saltándose la lista · impacto MEDIO · coste PEQUEÑO

- **Evidencia**: `handleEscape` mapea `ScreenRestoreBackup, ScreenRestoreConfirm` → `ScreenMainMenu` (`update.go:1148-1151`), mientras `handleRestoreConfirmKeys("esc")` → `ScreenRestoreBackup` (`update.go:2606`), rama inalcanzable. El test `update_test.go:235-244` vuelve a llamar al handler directamente y no lo detecta.

### 🟠 A7 — Emoji en menús y títulos, cuando el propio repo prohíbe emoji por las «cajas» en terminales sin fuente de emoji · impacto MEDIO · coste PEQUEÑO

- **Evidencia**: el comentario de `renderMainMenu` dice que quitó el emoji del título «*porque un terminal sin fuente de emoji lo dibujaba como una caja*» (`view.go:1899-1902`), pero las opciones siguen con emoji (`model.go:515-519` y siguientes) y `GetScreenTitle` devuelve títulos con emoji (`model.go:758-825`). La regla del repo y su guard: `view.go:3234-3242` y `trainer_e2e_test.go:3193`.

### 🟡 A8 — El panel de *Utilities* afirma «The dotfiles' own theme is not switchable here.» antes de haber abierto Utilities nunca · impacto MEDIO-BAJO · coste PEQUEÑO

- **Evidencia**: la entrada «absent» del panel (`panels.go:1026-1031`) se usa para `choiceUtilities` (`panels.go:743`), y las definiciones sólo se cargan en `dotfilesThemesCmdIfNeeded` (`update.go:297-303`), que se lanza al **abrir** la sección.
- **Resuelto**: el panel usa `dotfilesThemesPending` y dice *«The dotfiles' own theme has not been checked yet.»* antes de la primera lectura. Guard: `TestUtilitiesPanelSaysTheThemeHasNotBeenCheckedYet`.

### 🟡 A9 — `Esc` en el menú principal cierra toda la aplicación, aunque `Esc` se documenta como «volver» · impacto MEDIO-BAJO · coste PEQUEÑO

- **Evidencia**: `handleEscape`, `case ScreenMainMenu: m.Quitting = true; return m, tea.Quit` (`update.go:~1180-1183`), frente a `docs/tui-installer.md:545` (`Esc | Go back`) y al pie, que anuncia `[Space q] quit`.

### 🟡 A10 — `Backspace` funciona como «atrás» sólo en algunas pantallas · impacto BAJO · coste PEQUEÑO

- **Evidencia**: sólo `handleUtilitiesKeys` (`update.go:1443`), `handleWSLResourceKeys` (`update.go:1546`), `handleThemePickerKeys` (`update.go:1673`) y `handleSelectionKeys` (`update.go:1777`) incluyen `"backspace"`.

---

## B. Preferencias (a decidir, no son bugs)

- **B1** — El theme picker abre con el cursor en **Aplicar** (fila 0), mientras la revisión de refresh abre en **Cancel** (`update.go:767`, «Cancel is the safe default»). Decidir si el picker debe abrir en una fila segura.
- **B2** — El pie del welcome anuncia `[Space q] quit`, pero `Space` ahí **navega** al menú (`update.go:930-934` frente a `view.go:1174`).
- **B3** — Densidad de texto en las pantallas de utilidades: párrafos muy largos, recortados con `… and N more` (`view.go:1251-1330`).
- **B4** — La fila de pestañas del panel puede cambiar de **estilo** al añadirse *Last install* (hipótesis de umbral, ver D1).
- **B5** — Deshabilitar la lectura de definiciones en arranque es una decisión de coste (causa A2/A8).

---

## C. Lo que está bien (para saber dónde está la línea)

- **Marco único y coherente**: header + regla + cuerpo + regla + footer en todas las pantallas no-trainer (`view.go:42-60`), mismo componente de hints (`footerHints`, `installerHint`) y orden canónico nav → acción → volver (`view.go:63-96`).
- **El asistente es seguro y predecible**: OS y shell se preseleccionan con lo detectado, y sin detección el cursor abre en `noSelection` y Enter **se niega** en vez de responder por el usuario (`update.go:1234-1245`).
- **Los estados de espera que sí hablan**: terminal capabilities («*Reading what this terminal can do…*», `view.go:1204-1207`), WSL («*yet: reading…*», `view.go:1603`), panel live («*reading this machine…*», `metrics.go:175-184`), shell audit («*Measuring: N starts…*», `view.go:1770-1775`), instalación («*Estimating the time remaining…*» + ETA desde el modelo, `view.go:2600-2613`).
- **Los no-disponibles dicen por qué**, con texto en la sección y en el panel (`view.go:1360-1405`, `view.go:1788`).
- **Fallos dicen qué hacer**: pantalla de error con logs y `[r] retry` (`view.go:2784-2823`); `deadEnd` da el siguiente paso (`view.go:36-41`).
- **Render puro y testado**: dos renders del mismo modelo dan los mismos bytes (`panels_test.go:1629-1651`).
- **Aguanta tamaños pequeños**: guard de encaje a 60x20, 80x24, 160x50, 227x62 (`screen_coverage_test.go:168`, `:418-429`); el panel se colapsa por debajo de 124 columnas (`layout.go:31-37`).
- **Reversibilidad del tema**: cada cambio se registra y se deshace (`theme.json`), y `--dry-run` no escribe.

---

## D. Hipótesis (sin evidencia de ejecución — aparte)

- **H1** — `NewModel()` ejecuta detección síncrona **antes** del primer frame (`main.go` → `NewModel` → `system.Detect()`), y en macOS `checkXcode()` lanza `xcode-select -p` (`detect.go:280-283`). **No medido**.
- **H2** — El mecanismo exacto de los «dos renders» del ejemplo del usuario (`theme_preview.go:88-105` → `applyPreviewTheme`; `dotfilesThemeChangedMsg` en `update.go:713-729`), **deducido del código, no observado**.
- **H3** — El umbral exacto al que la fila de pestañas se trunca al añadir *Last install* (B4), plausible, no verificado.
- **H4** — No verificado en Termux real, pero la regla del repo y sus tests hacen el defecto lógico con independencia del entorno.

---

## Recuento

- **Defectos (A)**: **10** (A1–A10). **Preferencias (B)**: **5** (B1–B5).
- **Los tres primeros**: **A1** (Esc Utilities muerto), **A2** (Utilities miente al abrir), **A3** (el write de `.wslconfig` cambia la pantalla dos veces).

## El patrón sistémico (lo más importante del inventario)

**`handleEscape` no conoce varias pantallas** (A1, A6) → **las ramas `esc` de los handlers son código inalcanzable** → y **los tests llaman a los handlers directamente** (`update_test.go:1030-1037`, `update_test.go:235-244`), **saltándose el despacho real** → **una tecla muerta pasa en verde**. Es la misma clase de defecto que un guard que mide lo que no debe: **el test prueba el handler, no la tecla**.

---

## El patrón de este frente (A2, A3, A5, A8)

**La pantalla no puede decir algo falso, ni moverse bajo el cursor, mientras el trabajo está en curso.** Tres de los cuatro comparten un mecanismo: un estado **«en curso / sin comprobar» en el hueco del resultado**. A2 y A8 usan el mismo predicado `dotfilesThemesPending` para que la sección diga que está leyendo y el panel que no se ha comprobado todavía, en vez de afirmar el resultado antes de tenerlo. A3 marca `WSLState.Refreshing` y mantiene la tabla en pantalla mientras la relectura actualiza sus valores en su sitio, con la marca en el slot del notice. **A5 no es un hueco de contenido sino un problema de identidad del cursor**: la fila que aparece tarde desplaza los índices, así que el arreglo es anclar el cursor a la etiqueta que nombraba (`selectedOption`/`holdCursorOn`) — el mismo principio («no se mueve») por un mecanismo distinto, porque aquí no hay ningún texto que pueda mentir sobre un resultado que aún no existe.

---

## El defecto del colorscheme de Neovim (frente aparte — repo ↔ máquina)

- **Qué ve el usuario**: aplicó el tema **Nocturne** desde el instalador; la línea `opts.colorscheme = "nocturne"` quedó escrita en `~/.config/nvim/lua/plugins/colorscheme.lua`, pero `~/.config/nvim/colors/` **no existía** en su máquina. Neovim arrancaba roto con `E185: Cannot find color scheme` en cada inicio (y el aviso de `lualine` como consecuencia).
- **Qué nombre escribimos y dónde se buscó**: el interruptor escribió `nocturne` (tema aplicado confirmado por el usuario). En la máquina: `~/.config/nvim/colors/` **inexistente**; en `~/.local/share/nvim/lazy/` había `catppuccin/` y `kanagawa.nvim/`, pero ningún fichero de Nocturne. El fichero del repo `dotfiles-nvim/nvim/colors/nocturne.lua` existe y está bien generado: sólo faltaba la instalación.
- **Por qué no resolvió**: `nocturne` es un colorscheme **generado**, no de plugin, así que `:colorscheme nocturne` sólo resuelve si el fichero está en un `colors/` del runtimepath. El paso de Neovim copia `dotfiles-nvim/nvim/` (incluido `colors/`), pero el **interruptor de temas** sólo reescribe `colorscheme.lua` y **nunca dejaba el fichero generado en la máquina**. La brecha repo ↔ máquina estaba en el interruptor, no en la instalación.
- **Por qué el guard no lo cazó**: `TestTheNeovimColorschemeNamesResolve` comprueba que el nombre resuelva **desde el repositorio** (¿hay plugin o fichero generado en `dotfiles-nvim/nvim/colors/`?), no **desde la máquina del usuario**.
- **Resuelto** (`fix/nvim-colorscheme-reachable`): `reachableNvimColorscheme` (installer.go) instala el colorscheme generado en `~/.config/nvim/colors/<name>.lua` **antes** de escribir la línea, y sólo escribe el nombre de un plugin si hay un `colors/<name>.{lua,vim}` en el árbol de datos de Neovim de la máquina; si ninguno se puede alcanzar, deja la línea fuera y lo dice con el motivo en el aviso. Guard: `TestTheNvimColorschemeLineNamesOnlyAColorschemeTheMachineHas` (rojo primero: la línea se escribía sin el fichero → falla; con el arreglo → verde; quitando la garantía → vuelve a caer). Medido con Neovim real v0.12.5: sin el fichero → `E185: Cannot find color scheme 'nocturne'`; con `~/.config/nvim/colors/nocturne.lua` → `colors_name=nocturne`.

---

## E. Lo que se vio al MIRAR los frames (80×24, `fix/picker-layout`)

**Por qué esta sección existe**: las cinco pantallas de la sección de utilidades se habían verificado con **medidas** (filas, columnas, encaje a doce tamaños) y con **comportamiento**, pero **ninguna tenía un frame congelado**: nadie las había mirado. Ahora sí (`TestThemePickerGolden`, `TestThemePickerActivityGolden`, `TestThemePickerResultGolden`, `TestUtilitiesGolden`, `TestWSLResourcesGolden`, `TestShellAuditGolden`, `TestTerminalCapabilitiesGolden`, `teatest_test.go`). Lo de abajo es **lo que se ve al mirarlos**, con el frame pegado. **Nada de esto se arregló a ciegas**: se anota para el frente que lo posea.

Los frames son el render real a 80×24 (padding de 1 fila arriba, 2 columnas a los lados) con los códigos de color quitados.

### E1 — Sonda de terminal: tres frases pegadas sin separación, y «missing Why unknown» · impacto MEDIO · coste PEQUEÑO

- **Qué ve el usuario**: cada respuesta es `Etiqueta: valor — fuente. significado`, y el significado empieza **en minúscula después de un punto** (`. the themes are painted…`, `. copying reaches…`, `. a large redraw may flicker…`). En la fila del Nerd Font, además, la **etiqueta `Why unknown:` queda pegada a la frase anterior**: *«…if the Nerd Font is missing Why unknown: no terminal query reports…»* — se lee como una errata. Y en el `Check:` hay dos puntos dentro de la misma frase (*«Check: look at the marks on this screen: a box or a question mark…»*).
- **Por qué importa**: es la pantalla que explica qué puede y qué no puede hacer el terminal; el dato viene partido en tres y sin pausas, así que la primera lectura es un párrafo continuo, no tres hechos.
- **Evidencia**: `view.go` (`terminalCapabilitiesDescription` / las filas de `terminalAnswer`), frame de `TestTerminalCapabilitiesGolden`:

```
  Utilities
  ────────────────────────────────────────────────────────────────────────────

  🔌 Terminal capabilities
  Read-only: nothing here is changed and nothing is written. Each answer names
  where it came from, and an answer the probe could not determine is reported
  as unknown with a reason and a manual check.
  Colour depth: truecolor (24-bit) — COLORTERM=truecolor. the themes are
  painted with their exact colours
  Clipboard (OSC 52): supported — the terminal's reply to the Ms capability
  query. copying reaches the clipboard, including over SSH
  Synchronized output (mode 2026): not supported — the terminal's reply to the
  mode 2026 query. a large redraw may flicker or tear
  Nerd Font glyphs: unknown — not determined. icons may draw as boxes or
  question marks if the Nerd Font is missing Why unknown: no terminal query
  reports whether a Nerd Font is installed; the terminal draws the glyphs it
  is given. Check: look at the marks on this screen: a box or a question mark
  where an icon belongs means the font is missing.

  ▸ ← Back

  ────────────────────────────────────────────────────────────────────────────
  [Esc] back
```

### E2 — Sonda de terminal: el título cae una fila más abajo que el de sus pantallas hermanas · impacto BAJO · coste PEQUEÑO

- **Qué ve el usuario**: entre la regla y `🔌 Terminal capabilities` hay **una fila vacía**; en Utilities, WSL y la auditoría el título va **pegado a la regla**. La misma sección, dos alturas de título distintas.
- **Por qué es un problema**: el ojo lee la primera fila del cuerpo como el nombre de la pantalla; si esa fila cambia de sitio entre pantallas de la misma sección, el cambio se lee como un salto de maquetación.
- **Causa**: el cuerpo de esta pantalla es más corto que las filas que quedan, y `placeBody` lo **centra** (`view.go`, `placeBody`/`placeBodyTopMarginMax`): las filas sobrantes se reparten arriba y abajo.
- **Evidencia**: mismo frame que E1 (la primera línea tras la regla está vacía).

### E3 — Utilities: 14 de 19 filas son prosa, y `… and 8 more` esconde la descripción de las filas que sí están en pantalla · impacto MEDIO · coste MEDIO

- **Qué ve el usuario**: la sección abre con cinco párrafos (qué no hay, por qué no hay WSL, para qué sirve medir el shell, para qué sirve la sonda) y **corta con `… and 8 more`** justo antes de la lista. Es decir: la mitad del texto que describe las filas no se puede leer **nunca** en 80×24, y las filas que sí se ven quedan sin su explicación.
- **Por qué importa**: la prosa es el 74 % del cuerpo y las acciones son 4 filas separadas por 3 reglas; quien entra a hacer algo tiene que bajar la vista por 14 líneas de explicación para llegar a 4 filas, y no puede leer por qué existen.
- **Evidencia**: frame de `TestUtilitiesGolden`:

```
  Utilities
  ────────────────────────────────────────────────────────────────────────────
  🧰 Utilities
  No desktop theme switch is available here. Switching needs a GNOME, KDE
  Plasma or macOS session with the tool that changes its theme on PATH; a
  server, Termux or a plain terminal has none, so no switch is offered.
  WSL resources are not adjustable here: .wslconfig is a Windows file read
  only by WSL, and Linux does not run it. The utility is offered only on a WSL
  host.
  Measure the shell's startup starts zsh -i -c exit the way a terminal does,
  several times, and reports the median with the range -- and on zsh names the
  functions zprof blames for it. It changes nothing: no startup file is
  … and 8 more

  ▸ Change the dotfiles theme
  ────────────────────────────────────────────────────────────────────────────
    Measure the shell's startup
  ────────────────────────────────────────────────────────────────────────────
    Report the terminal capabilities
  ────────────────────────────────────────────────────────────────────────────
    ← Back
  ────────────────────────────────────────────────────────────────────────────
  ↑/k up • ↓/j down • [Enter] select • [Esc] back
```

- **Dos cosas más que el frame deja ver** (mismo frame): la **primera frase es un hecho negativo** (*«No desktop theme switch is available here»*) y **justo debajo se ofrece `Change the dotfiles theme`** — dos «theme» distintos en una pantalla, sin una línea que los distinga (es la zona de A2/A8); y **la fila de cada opción va separada por una regla de ancho completo** (3 reglas para 4 filas), mientras el selector de temas agrupa sólo la zona de acciones: el mismo componente de lista se ve denso en una pantalla y en caja en la otra.

### E4 — Auditoría del shell: los mismos números dos veces, y `… and 20 more` esconde justo la atribución · impacto MEDIO · coste PEQUEÑO-MEDIO

- **Qué ve el usuario**: arriba, en prosa, `Finished starts: 4 of 5. Median 924 ms, fastest 872 ms, slowest 977 ms.` y `The starts, in the order they ran: 977 ms, 915 ms, 872 ms, 932 ms.`; abajo, como filas, `Median of 4 of 5 runs: 924 ms`, `Range: 872 ms to 977 ms`, `1 of 5 runs did not finish: 1 timed out at 10 s`. **El mismo dato en dos sitios**, y el corte `… and 20 more` cae antes de la tabla de funciones, que es lo único que responde a *«¿por qué tarda?»*.
- **Por qué importa**: la pantalla gasta 9 filas repitiendo lo que las 4 filas de abajo ya dicen, y esconde 20 líneas — la atribución por función (`compdump`, `compdef`, …) está cortada al llegar: sólo se ve `Slowest function: compdump` en una fila.
- **Evidencia**: frame de `TestShellAuditGolden`:

```
  Utilities
  ────────────────────────────────────────────────────────────────────────────
  ⏱️ The shell's startup
  zsh is started the way a terminal starts it -- `zsh -i -c exit`, each start
  bounded by 10 s -- so the number describes the start you wait through when
  you open a terminal. The measurement is 5 such starts, and the median is the
  middle one: not an average, and not a single run.
  Finished starts: 4 of 5. Median 924 ms, fastest 872 ms, slowest 977 ms.
  Every completed start is listed so the number can be reproduced by hand. The
  starts, in the order they ran: 977 ms, 915 ms, 872 ms, 932 ms.
  1 of the 5 starts did not finish inside 10 s and were killed at that bound.
  … and 20 more

  ▸ Measure the shell's startup
  ────────────────────────────────────────────────────────────────────────────
    Median of 4 of 5 runs: 924 ms
    Range: 872 ms to 977 ms
    1 of 5 runs did not finish: 1 timed out at 10 s
    Slowest function: compdump (725.25 ms self, 1 call(s))
  ────────────────────────────────────────────────────────────────────────────
    ← Back
  ────────────────────────────────────────────────────────────────────────────
  ↑/k up • ↓/j down • [Enter] select • [Esc] back
```

- **Y qué se lee primero**: `zsh is started the way a terminal starts it` — el método. El número, que es lo que el usuario vino a ver, está en la sexta fila. La primera fila de la lista (`▸ Measure the shell's startup`) además invita a **volver a medir** (5 arranques con un tope de 10 s cada uno) sin confirmación previa.

### E5 — WSL resources: la recomendación se explica antes de enseñarse, y el pie se parte en dos líneas desalineadas · impacto MEDIO-BAJO · coste PEQUEÑO

- **Qué ve el usuario**: 11 líneas de prosa (los números del host, la recomendación y **la derivación entera** de cómo se calcula, cortada con `… and 10 more`) antes de las tres filas editables. La explicación de la fórmula ocupa más que los valores que ajusta.
- **Y el pie**: a 80 columnas los hints no caben en una línea y el segundo (`[Esc] back`) **empieza en la columna 3**, no alineado con el primero, que empieza después del separador `↑/k up •`.
- **Evidencia**: frame de `TestWSLResourcesGolden`:

```
  Utilities
  ────────────────────────────────────────────────────────────────────────────
  🖥️ WSL resources
  Windows reports 16384 MiB of RAM and 8 logical processors; both numbers come
  from the same host detector the installation step reads them through.
  Recommended for this host: memory 8192 MB, processors 8 and swap 2048 MB.
  The proportions are the ones the installation step uses, taken from the
  shipped template rather than typed here: memory is half the host's RAM
  rounded down to 512 MB, but never so much that Windows keeps less than 2
  GiB; processors are every logical CPU the host reports; swap is a quarter of
  that memory, rounded down to the same step.
  … and 10 more

  ▸ Memory: 8192 MB
    Processors: 8
    Swap: 2048 MB
  ────────────────────────────────────────────────────────────────────────────
    Write the .wslconfig
  ────────────────────────────────────────────────────────────────────────────
    ← Back
  ────────────────────────────────────────────────────────────────────────────
  ↑/k up • ↓/j down • [←/→] adjust • [r] recommended • [Enter] select
  [Esc] back
```

### E6 — Selector de temas: el informe sólo cabe a medias en 80×24, y no hay forma de leer el resto · impacto MEDIO · **trade consciente, no defecto de este cambio**

- **Qué ve el usuario** (estado con la actividad ya asentada, frame de `TestThemePickerResultGolden`): debajo de la lista, cinco filas: tres de método, la primera herramienta y `… and 7 more`. Es decir: **el usuario sabe que hay siete líneas más y no puede leerlas en esa pantalla**.
- **Por qué se acepta así**: el encargo pedía que **la lista no se aplaste** y que el estado vaya **debajo** de ella. A 80×24 el cuerpo tiene 19 filas; la lista completa ocupa 11 y la preview 1, así que la ranura del informe no puede pasar de 5 sin recortar la lista — que es justo lo que se venía a arreglar (antes: lista de 5 filas y el informe con 11). Se prefiere el dato que el usuario está recorriendo.
- **Lo que queda abierto** (para el frente que quiera cerrarlo): el informe largo **no tiene segundo sitio** al que ir en esa pantalla. Opciones que no se toman aquí: una fila-resumen en la ranura y el detalle en otra vista; o que el bloque de método no ocupe la ranura cuando el informe no cabe.
- **Evidencia**: frame de `TestThemePickerResultGolden` (arriba del todo de esta sección está el «antes», que se pega en el informe del cambio).

### E7 — Selector de temas: al pulsar, la lista **sube** las filas que deja la descripción · impacto MEDIO-BAJO · límite declarado

- **Qué ve el usuario**: la descripción (4 filas en 80×24) es lo único que cede al pulsar, así que la lista sube ésas 4 filas; el cursor sigue **en el mismo tema**, 4 líneas más arriba. Desde la pulsación hasta que llega el resultado **la fila del cursor, el alto del marco y las filas del informe no se mueven**, pero **la tira de preview sí**: durante la espera queda en la fila 18 y al llegar el resultado baja a la 22. La causa es que las filas que la espera no usa (4 de las 5 de la ranura) se dibujan al pie del cuerpo, **por debajo de la preview**.
- **Por qué se acepta**: en 80×24 no caben a la vez la lista entera (11), la descripción (4) y la ranura del informe (5): son 20 filas para 16 disponibles. Ceder la descripción es lo que mantiene la lista entera y el informe con sitio, y dejar el hueco al pie es lo que hace que la espera se lea como una línea pegada a la lista en vez de como un boquete en medio. De las dos piezas que podrían moverse —el hueco, si fuera entre la actividad y la preview, o la preview— se mueve la preview, que queda por debajo de lo que el que elige está leyendo. El comportamiento anterior tenía el mismo salto de descripción y, además, aplastaba la lista a 5 filas.
- **Evidencia**: el frame «antes» (lista de 5 filas, `Showing 1-5 of 11`, la actividad **arriba** y trece filas vacías en medio) y el «después» (lista entera, `Applying the Nocturne theme…` justo debajo):

```text
# ANTES — view.go de HEAD (themeActivityMenuRows = 5), TestThemePickerActivityGolden

  dotfiles                                                   Showing 1-5 of 11
  ────────────────────────────────────────────────────────────────────────────
  🎨 Change the dotfiles theme
  Applying the Nocturne theme…

  (trece filas vacías)

  ▸ Apply the Catppuccin Latte theme
    Apply the Catppuccin Mocha theme
    Apply the dotfiles theme
    Apply the Everforest theme
    Apply the Kanagawa theme
  Preview (nothing applied) — Catppuccin Latte:
  ────────────────────────────────────────────────────────────────────────────
  ↑/k up • ↓/j down • [Enter] select • [Esc] back
```

```text
# DESPUÉS — view.go de este frente, TestThemePickerActivityGolden

  dotfiles
  ────────────────────────────────────────────────────────────────────────────
  🎨 Change the dotfiles theme

  ▸ Apply the Catppuccin Latte theme
    Apply the Catppuccin Mocha theme
    Apply the dotfiles theme
    Apply the Everforest theme
    Apply the Kanagawa theme
    Apply the Nocturne theme
    Apply the Rosé Pine theme
  ────────────────────────────────────────────────────────────────────────────
    Refresh outdated theme files
  ────────────────────────────────────────────────────────────────────────────
    ← Back
  Applying the Nocturne theme…
  Preview (nothing applied) — Catppuccin Latte:

  (cuatro filas reservadas para el informe)

  ────────────────────────────────────────────────────────────────────────────
  ↑/k up • ↓/j down • [Enter] select • [Esc] back
```

- **El frame del resultado** (`TestThemePickerResultGolden`) enseña adónde van esas cuatro filas y por qué la preview baja: el informe ocupa las filas 17-21 y la preview vuelve a la 22.

```text
    ← Back
  The theme is written to every file it owns. Writing a file is not the same
  as the tool showing it, so here is each tool the switch painted, what the
  installer did for it, and what is left:
  ✓ Alacritty — Alacritty watches its config and reloads it live, so the new
  … and 7 more
  Preview (nothing applied) — Catppuccin Latte:
  ────────────────────────────────────────────────────────────────────────────
  ↑/k up • ↓/j down • [Enter] select • [Esc] back
```

### E8 — El header dice «dotfiles» en el selector y «Utilities» en las utilidades · impacto BAJO · coste PEQUEÑO

- **Qué ve el usuario**: la fila del header (arriba a la izquierda) es el **nombre de la sección** en Utilities, WSL, auditoría y sonda (`Utilities`), y el **nombre del programa** en el selector (`dotfiles`). En la sección Utilities el nombre aparece además **dos veces** (`Utilities` en el header y `🧰 Utilities` en el título), con dos estilos distintos.
- **Por qué es una observación y no un arreglo**: `headerName()` decide ese texto, y cambiarlo mueve los frames de todas las pantallas; no es de este frente.
- **Evidencia**: los frames de E1–E5 (header `Utilities` + título propio) frente a los del selector (header `dotfiles` + `🎨 Change the dotfiles theme`).
