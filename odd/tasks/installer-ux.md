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

## El patrón de este frente (A2, A3, A5, A8)

**La pantalla no puede decir algo falso, ni moverse bajo el cursor, mientras el trabajo está en curso.** Tres de los cuatro comparten un mecanismo: un estado **«en curso / sin comprobar» en el hueco del resultado**. A2 y A8 usan el mismo predicado `dotfilesThemesPending` para que la sección diga que está leyendo y el panel que no se ha comprobado todavía, en vez de afirmar el resultado antes de tenerlo. A3 marca `WSLState.Refreshing` y mantiene la tabla en pantalla mientras la relectura actualiza sus valores en su sitio, con la marca en el slot del notice. **A5 no es un hueco de contenido sino un problema de identidad del cursor**: la fila que aparece tarde desplaza los índices, así que el arreglo es anclar el cursor a la etiqueta que nombraba (`selectedOption`/`holdCursorOn`) — el mismo principio («no se mueve») por un mecanismo distinto, porque aquí no hay ningún texto que pueda mentir sobre un resultado que aún no existe.

---

## El defecto del colorscheme de Neovim (frente aparte — repo ↔ máquina)

- **Qué ve el usuario**: aplicó el tema **Nocturne** desde el instalador; la línea `opts.colorscheme = "nocturne"` quedó escrita en `~/.config/nvim/lua/plugins/colorscheme.lua`, pero `~/.config/nvim/colors/` **no existía** en su máquina. Neovim arrancaba roto con `E185: Cannot find color scheme` en cada inicio (y el aviso de `lualine` como consecuencia).
- **Qué nombre escribimos y dónde se buscó**: el interruptor escribió `nocturne` (tema aplicado confirmado por el usuario). En la máquina: `~/.config/nvim/colors/` **inexistente**; en `~/.local/share/nvim/lazy/` había `catppuccin/` y `kanagawa.nvim/`, pero ningún fichero de Nocturne. El fichero del repo `dotfiles-nvim/nvim/colors/nocturne.lua` existe y está bien generado: sólo faltaba la instalación.
- **Por qué no resolvió**: `nocturne` es un colorscheme **generado**, no de plugin, así que `:colorscheme nocturne` sólo resuelve si el fichero está en un `colors/` del runtimepath. El paso de Neovim copia `dotfiles-nvim/nvim/` (incluido `colors/`), pero el **interruptor de temas** sólo reescribe `colorscheme.lua` y **nunca dejaba el fichero generado en la máquina**. La brecha repo ↔ máquina estaba en el interruptor, no en la instalación.
- **Por qué el guard no lo cazó**: `TestTheNeovimColorschemeNamesResolve` comprueba que el nombre resuelva **desde el repositorio** (¿hay plugin o fichero generado en `dotfiles-nvim/nvim/colors/`?), no **desde la máquina del usuario**.
- **Resuelto** (`fix/nvim-colorscheme-reachable`): `reachableNvimColorscheme` (installer.go) instala el colorscheme generado en `~/.config/nvim/colors/<name>.lua` **antes** de escribir la línea, y sólo escribe el nombre de un plugin si hay un `colors/<name>.{lua,vim}` en el árbol de datos de Neovim de la máquina; si ninguno se puede alcanzar, deja la línea fuera y lo dice con el motivo en el aviso. Guard: `TestTheNvimColorschemeLineNamesOnlyAColorschemeTheMachineHas` (rojo primero: la línea se escribía sin el fichero → falla; con el arreglo → verde; quitando la garantía → vuelve a caer). Medido con Neovim real v0.12.5: sin el fichero → `E185: Cannot find color scheme 'nocturne'`; con `~/.config/nvim/colors/nocturne.lua` → `colors_name=nocturne`.
- **El DEFAULT era de la misma clase** (`fix/nvim-default-theme`): el fichero que el paso de Neovim copia tal cual traía `colorscheme = "kanagawa"` (`DefaultTheme: "kanagawa"` en `themeActiveArtifacts`), y **`kanagawa` es un colorscheme de plugin** — un extra que una máquina puede no tener. El `theme` de lualine estaba **escrito a mano** en `dotfiles-nvim/nvim/lua/plugins/ui.lua` ("Match the configured colorscheme"), así que el aviso del usuario salía por partida doble: el default pedía un plugin y lualine nombraba un tema que sólo ese plugin trae. **Por qué el default importa aunque el interruptor se defienda**: la aplicación ya se negaba a escribir un nombre inalcanzable, pero el default se salta esa defensa — es lo que un install nuevo escribe antes de que nadie toque el interruptor.
- **Resuelto**: (1) el default del artefacto nvim vuelve a `defaultThemeID` (`dotfiles`), que es **el único colorscheme que este repo genera e instala en la misma copia de config** → siempre resuelve sin plugins; el bloque commiteado pasa a `colorscheme = "dotfiles"` y el guard de generación lo verifica byte a byte sin `-update`. (2) la línea de lualine se borra: LazyVim ya pone `options.theme = "auto"` (leído en `~/.local/share/nvim/lazy/LazyVim/lua/lazyvim/plugins/ui.lua:90`) y lualine deriva el tema del colorscheme activo (`lualine/themes/auto.lua` lee `vim.g.colors_name`), así que escribirlo a mano era innecesario y sólo podía desincronizarse. (3) el anclaje de adopción pasa de `colorscheme = "kanagawa"` a `colorscheme = ` (la asignación, no el nombre): la línea que el interruptor reescribe lleva el tema que se haya aplicado, y un nombre fijo sólo adopta ese uno.
- **Guards y dientes** (`installer/internal/tui/install_paths_test.go`): `TestTheCommittedNvimColorschemeResolvesWithoutAPlugin` (rojo: el bloque commiteado decía kanagawa → falla nombrando `Theme kanagawa not found`/E185), `TestTheNvimStatuslineLeavesTheThemeToLazyVim` (rojo: `ui.lua:86` `opts.options.theme = "kanagawa"` → falla), `TestTheNvimAdoptionAnchorMatchesTheLineNotTheTheme` (rojo: anclaje fijo → no adopta un fichero legado que diga `nocturne` ni `dotfiles`). Volviendo a poner cada regresión, cada uno cae.
- **Medido con Neovim real v0.12.5** (`--clean --headless`, sin tocar la config del usuario): `colorscheme dotfiles` con el `colors/` del repo en el runtimepath → `colors_name=dotfiles`, **sin E185**; `colorscheme kanagawa` sin el plugin → `E185: Cannot find color scheme 'kanagawa'`; lualine con `theme='kanagawa'` y sin el plugin → el aviso exacto del usuario, `Theme `kanagawa` not found, falling back to `auto`. Check if spelling is right.`; lualine con `theme='auto'` sobre `colorscheme dotfiles` → **cero avisos**; y `dofile` del `ui.lua` real sobre las opts de LazyVim → `options.theme=auto`, `icons_enabled=true`.
