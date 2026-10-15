# Vim Mastery Trainer - Especificación Completa

## Tabla de Contenidos

- [Contexto del Proyecto](#contexto-del-proyecto)
- [Arquitectura Existente](#arquitectura-existente)
- [Concepto: RPG de Vim](#concepto-rpg-de-vim)
- [Estructura de Progresión](#estructura-de-progresión)
- [Módulos de Entrenamiento](#módulos-de-entrenamiento)
- [UI Mockups](#ui-mockups)
- [Boss Fights](#boss-fights)
- [Estructura de Datos](#estructura-de-datos)
- [Estructura de Archivos](#estructura-de-archivos)
- [Integración con TUI Existente](#integración-con-tui-existente)
- [Componentes Bubbletea](#componentes-bubbletea)
- [Comandos de Desarrollo](#comandos-de-desarrollo)
- [Plan de Implementación (MVP)](#plan-de-implementación-mvp)
- [Estilo de Código](#estilo-de-código)

---

## Contexto del Proyecto

Juego de entrenamiento de Vim estilo RPG integrado al TUI installer de dotfiles. Construido con **Go + Bubbletea** (Charmbracelet).

**Ubicación**: `installer/` | **Nueva opción de menú**: "🎮 Vim Mastery Trainer"

## Arquitectura Existente

```text
installer/
├── cmd/dotfiles/main.go
├── internal/
│   ├── system/          # Detección OS, ejecución comandos
│   └── tui/
│       ├── installer.go # Lógica de instalación
│       ├── model.go     # Model principal (Bubbletea)
│       ├── update.go    # Update handlers
│       ├── view.go      # Views
│       ├── styles.go    # Lipgloss styles
│       ├── keymaps*.go  # Pantallas de keymaps
│       └── constants.go # Screens enum
```

---

## Concepto: RPG de Vim

Trainer estilo **keybr.com para Vim** con progresión RPG:

| Elemento | Descripción |
|----------|-------------|
| Módulos | Cada módulo es un "dungeon" |
| Progresión | Lecciones → Práctica → Jefe Final |
| Desbloqueo | Derrotar jefe desbloquea siguiente módulo |
| Características | Stats persistentes, spaced repetition, combos |

---

## Estructura de Progresión

```text
📖 LECCIONES (Tutorial)
    │
    │  Ejercicios guiados con explicación
    │  Sin timer estricto, enfoque en aprender
    │  100% para desbloquear práctica
    │
    ▼
🎯 PRÁCTICA (Grinding)
    │
    │  Ejercicios aleatorios del módulo, sesgados al siguiente nivel a aprender
    │  Con timer, scoring, streaks
    │  80% de los últimos 20 intentos (y al menos 10 intentos) para desbloquear jefe
    │  Lo dominado vuelve a los 12 h para repaso
    │
    ▼
👹 JEFE FINAL (Boss Fight)
    │
    │  Ejercicio épico que combina TODO del módulo
    │  Timer calibrado a la forma del paso, múltiples pasos, 3 vidas
    │  La pelea mide su propio tiempo y guarda el mejor
    │  Derrotarlo desbloquea siguiente sección
    │
    ▼
🔓 SIGUIENTE SECCIÓN DESBLOQUEADA
```

---

## Módulos de Entrenamiento

### 🏃 Movimientos Horizontales

| Comando | Descripción |
|---------|-------------|
| `w`, `W` | Siguiente palabra / PALABRA |
| `e`, `E` | Final de palabra / PALABRA |
| `b`, `B` | Inicio palabra anterior / PALABRA |
| `ge`, `gE` | Final palabra anterior / PALABRA |
| `f{c}` | Hasta carácter (inclusive) |
| `F{c}` | Hasta carácter hacia atrás |
| `t{c}` | Hasta carácter (exclusive) |
| `T{c}` | Hasta carácter hacia atrás (exclusive) |
| `;` | Repetir f/F/t/T |
| `,` | Repetir f/F/t/T en dirección opuesta |
| `0` | Inicio de línea |
| `$` | Final de línea |
| `^` | Primer carácter no-blanco |

### 📐 Movimientos Verticales

| Comando | Descripción |
|---------|-------------|
| `j`, `k` | Abajo / Arriba |
| `gg` | Primera línea |
| `G` | Última línea |
| `{n}G` | Ir a línea n |
| `{`, `}` | Párrafo anterior / siguiente |
| `H`, `M`, `L` | Top / Middle / Bottom de pantalla |
| `ctrl+d` | Media página abajo |
| `ctrl+u` | Media página arriba |
| `ctrl+f` | Página completa abajo |
| `ctrl+b` | Página completa arriba |

### 🎯 Text Objects

**CHANGE (c):**

| Comando | Descripción |
|---------|-------------|
| `ciw`, `caw` | Change inner/around word |
| `ci"`, `ca"` | Change inner/around "quotes" |
| `ci'`, `ca'` | Change inner/around 'quotes' |
| `ci{`, `ca{` | Change inner/around {braces} |
| `ci(`, `ca(` | Change inner/around (parens) |
| `ci[`, `ca[` | Change inner/around [brackets] |
| `cit`, `cat` | Change inner/around \<tags\> |
| `` ci` ``, `` ca` `` | Change inner/around \`backticks\` |

**Otros operadores:** Los mismos patterns aplican para `d` (delete), `y` (yank) y `v` (visual select).
Ejemplo: `diw`, `daw`, `yiw`, `viw`, etc.

### 🔁 Change & Repeat (El Flujo Mágico)

| Comando | Descripción |
|---------|-------------|
| `*` | Buscar palabra bajo cursor (forward) |
| `#` | Buscar palabra bajo cursor (backward) |
| `n`, `N` | Siguiente / anterior match |
| `gn` | Seleccionar próximo match (visual) |
| `cgn` | Cambiar próximo match |
| `dgn` | Borrar próximo match |
| `.` | Repetir último cambio |

**Combo Mágico (ventaja vs `:%s` - podés ELEGIR cuáles reemplazar):**

1. Cursor sobre palabra
2. `*` → Busca la palabra
3. `cgn` → Cambia el próximo match
4. `{texto}` → Escribí el reemplazo
5. `<Esc>` → Volver a normal
6. `.` → Repetir (siguiente match)
7. `n` → Saltear uno si querés
8. `.` → Seguir reemplazando

### 🔄 Sustitución (%s)

| Comando | Descripción |
|---------|-------------|
| `:s/foo/bar/` | Línea actual, primera ocurrencia |
| `:s/foo/bar/g` | Línea actual, todas las ocurrencias |
| `:%s/foo/bar/g` | Todo el archivo |
| `:%s/foo/bar/gc` | Todo el archivo, con confirmación |
| `:10,20s/foo/bar/g` | Rango de líneas (10-20) |
| `:'<,'>s/foo/bar/g` | Selección visual |
| `:s/foo/bar/i` | Case insensitive |
| `:s/foo/bar/I` | Case sensitive (forzado) |

**Patrones útiles:**

| Comando | Descripción |
|---------|-------------|
| `:%s/\s\+$//g` | Eliminar trailing whitespace |
| `:%s/foo/bar/gI` | Reemplazar exacto (case sensitive) |
| `:%s/\<foo\>/bar/g` | Solo palabras completas |

### 🔍 Regex & Vimgrep

**Búsqueda básica:**

| Comando | Descripción |
|---------|-------------|
| `/pattern` | Buscar hacia adelante |
| `?pattern` | Buscar hacia atrás |
| `n`, `N` | Siguiente / anterior match |
| `*` | Buscar palabra bajo cursor |

**Regex:**

| Comando | Descripción |
|---------|-------------|
| `/\<word\>` | Word boundaries (palabra completa) |
| `/pattern\c` | Case insensitive |
| `/pattern\C` | Case sensitive |
| `\v` | Very magic (menos escapes) |

**Very Magic Mode (`\v`):**

| Comando | Descripción |
|---------|-------------|
| `/\vfunction\s+\w+` | Buscar "function" + espacio + nombre |
| `/\v(\w+)@(\w+)` | Capturar grupos para email |

**Vimgrep:**

| Comando | Descripción |
|---------|-------------|
| `:vimgrep /pattern/g **/*.ts` | Buscar en todos los .ts |
| `:vimgrep /TODO/g **/*` | Buscar TODOs en proyecto |
| `:cnext`, `:cprev` | Navegar resultados |
| `:copen` | Abrir quickfix list |
| `:cclose` | Cerrar quickfix |

**Caracteres a escapar:** Sin `\v`: `. * [ ] ^ $ \ / ~` | Con `\v`: solo `/ \`

### 🎪 Macros

**Grabar:**

| Comando | Descripción |
|---------|-------------|
| `qa` | Empezar a grabar en registro 'a' |
| `{acciones}` | Las acciones que querés repetir |
| `q` | Parar de grabar |

**Ejecutar:**

| Comando | Descripción |
|---------|-------------|
| `@a` | Ejecutar macro del registro 'a' |
| `@@` | Repetir última macro ejecutada |
| `5@a` | Ejecutar macro 5 veces |
| `:5,10normal @a` | Ejecutar en líneas 5-10 |

**Tips:**
- Empezar macro con `0` o `^` (posición consistente)
- Terminar con `j` (ir a siguiente línea)
- Usar `f`/`t` en vez de `w` para mayor precisión

**Ejemplo - Convertir lista a array:**

```text
Antes:              Macro: qa0i"<Esc>A",<Esc>jq      Después de @a@@:
  item1                                                "item1",
  item2                                                "item2",
  item3                                                "item3",
```

### 📝 Editing & Undo

| Comando | Descripción |
|---------|-------------|
| `i`, `a`, `I`, `A`, `o`, `O` | Entrar en modo inserción |
| `<Esc>` | Volver a modo normal |
| `u`, `Ctrl-r` | Deshacer / rehacer |
| `dd`, `yy`, `p`, `P` | Borrar, copiar y pegar líneas |
| `>>`, `<<`, `>motion` | Indentar |
| `x`, `D`, `%`, marcas | Otras ediciones |

### 📋 Registers & Indentation

| Comando | Descripción |
|---------|-------------|
| `yy`, `yiw`, `y$`, `yj` | Copiar en el registro sin nombre |
| `"a`-`"z` | Registros con nombre |
| `"0` | Registro que solo guarda lo copiado |
| `p`, `P` | Pegar antes / después del cursor |
| `dd`, `x`, `D` | Borrar hacia el registro |
| `>>`, `<<` | Indentar líneas |

### Cómo se juzga un ejercicio: movimiento o buffer

Cada ejercicio se comprueba con uno de dos jueces, y el propio ejercicio
declara cuál usa. Un ejercicio de movimiento se compara por dónde queda el
cursor y, en los operadores, qué selección deja. Un ejercicio juzgado por buffer
se compara por el texto completo que la respuesta deja, más el cursor y el modo
finales; las teclas usadas no importan, solo el resultado.

| Juez | Qué compara | Cuándo se usa |
|------|-------------|----------------|
| Movimiento (simulador) | La posición final del cursor y la selección que dejan los operadores | Movimientos, text objects y todo comando cuyo efecto visible es dónde queda el cursor |
| Buffer (motor de edición) | El texto del buffer, el cursor y el modo finales | Deshacer/rehacer, pegar, registros, indentación y todo comando cuyo efecto vive en el texto |

Un ejercicio pide el juez de buffer con `BufferVerified: true`; el valor por
defecto conserva el juez de movimiento, así que el corpus existente no cambia.
Cuando la respuesta no coincide, la pantalla de resultado muestra el buffer
esperado junto al obtenido para que la diferencia se vea.

La regla de contenido es que un ejercicio solo puede prometer un resultado que su
juez pueda comprobar. Undo, pegar, los registros y la indentación se comprueban
por el buffer. Los comandos de Ex (como `:g/...`, `:v/...` o `:normal`) que el
motor todavía no ejecuta se enseñan como una lección de tecleo: la misión dice
que el entrenador comprueba las teclas y no afirma un resultado que no puede
verificar.

---

## UI Mockups

### Menú Principal del Trainer

```text
┌─────────────────────────────────────────────────────────────────┐
│                    🎮 VIM MASTERY TRAINER                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   🏃 HORIZONTAL MOTIONS                                        │
│   ├── 📖 Lecciones      ████████████ 100%  ✓                   │
│   ├── 🎯 Práctica       ████████░░░░  70%                      │
│   └── 👹 Jefe Final     🔒 (completa práctica al 80%)          │
│                                                                 │
│   📐 VERTICAL MOTIONS                                          │
│   ├── 📖 Lecciones      ██████░░░░░░  50%                      │
│   ├── 🎯 Práctica       🔒                                     │
│   └── 👹 Jefe Final     🔒                                     │
│                                                                 │
│   🎯 TEXT OBJECTS                                              │
│   ├── 📖 Lecciones      🔒 (derrota jefe anterior)             │
│   ├── 🎯 Práctica       🔒                                     │
│   └── 👹 Jefe Final     🔒                                     │
│                                                                 │
│   🔁 CHANGE & REPEAT    🔒                                     │
│   🔄 SUSTITUCIÓN        🔒                                     │
│   🔍 REGEX & VIMGREP    🔒                                     │
│   🎪 MACROS             🔒                                     │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│   ⚔️  Jefes derrotados: 1/7    🏆 Score: 2,340                  │
└─────────────────────────────────────────────────────────────────┘
```

### Pantalla de Ejercicio (Lección/Práctica)

```text
┌─────────────────────────────────────────────────────────────────┐
│   🎯 TEXT OBJECTS    Nivel 5/10    🔥 Racha: 7    Score: 340   │
│   ████████████░░░░░░░░ 60%                                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   CÓDIGO:                                                       │
│   ┌───────────────────────────────────────────────────────────┐│
│   │ 1  const config = {                                      ││
│   │ 2    name: "█dotfiles",                                 ││
│   │ 3    theme: "dark"                                       ││
│   │ 4  };                                                    ││
│   └───────────────────────────────────────────────────────────┘│
│                                                                 │
│   MISIÓN: Cambiá el contenido entre las comillas por "pro"     │
│           (el cursor está en la 'g' de dotfiles)              │
│                                                                 │
│                         ⏱️  5.2s                                │
├─────────────────────────────────────────────────────────────────┤
│   Tu input: ci"_                                                │
│                                                                 │
│   💡 Pista en 3s...                                            │
└─────────────────────────────────────────────────────────────────┘
```

### Pantalla de Resultado

```text
┌─────────────────────────────────────────────────────────────────┐
│   ✅ CORRECTO!  +50pts  ⚡ 2.3s                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   Tu respuesta: ci"pro<Esc>                                    │
│   Solución óptima: ci"pro<Esc> ✓                               │
│                                                                 │
│   📝 EXPLICACIÓN:                                              │
│   ci" = Change Inside " (comillas)                             │
│   - c = change (borra y entra en insert mode)                  │
│   - i" = inner quotes (contenido entre comillas)               │
│                                                                 │
│   También válido: f"ci", vi"c                                  │
│                                                                 │
│   [Enter] Siguiente    [r] Repetir    [q] Menú                 │
└─────────────────────────────────────────────────────────────────┘
```

### Boss Fight

```text
┌─────────────────────────────────────────────────────────────────┐
│   👹 JEFE FINAL: The Horizontal Nightmare    ❤️ ❤️ ❤️           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   CÓDIGO:                                                       │
│   ┌───────────────────────────────────────────────────────────┐│
│   │ 1  const█userName = getUser().name.firstName.toUpper();  ││
│   └───────────────────────────────────────────────────────────┘│
│                                                                 │
│   CADENA DE MISIONES:                         Ronda 1/5        │
│                                                                 │
│   ➤ Mové al final de "getUser"        ⏱️ 3s                    │
│   ○ Mové al inicio de "firstName"                              │
│   ○ Borrá "toUpper"                                            │
│   ○ Mové a la última 'e' de la línea                           │
│   ○ Volvé al inicio de la línea                                │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│   > fe_                                                         │
│                                                                 │
│   ⚡ Combo x2    👹 HP: ███████░░░                              │
└─────────────────────────────────────────────────────────────────┘
```

### Boss Derrotado

```text
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│                     👹 JEFE DERROTADO! 👹                       │
│                                                                 │
│              ░░░░░▒▒▒▒▓▓▓▓████████▓▓▓▓▒▒▒▒░░░░░               │
│                                                                 │
│                   🏆 +500 PUNTOS 🏆                             │
│                                                                 │
│              ⏱️  Tiempo: 34.2s (Record: 28.1s)                  │
│              ❤️  Vidas restantes: 2/3                           │
│              ⚡ Mejor combo: x4                                  │
│                                                                 │
│         ┌─────────────────────────────────────────────┐       │
│         │  🔓 TEXT OBJECTS desbloqueado!              │       │
│         └─────────────────────────────────────────────┘       │
│                                                                 │
│   [Enter] Continuar    [r] Reintentar (mejorar record)         │
└─────────────────────────────────────────────────────────────────┘
```

---

## Boss Fights

### Bosses por Módulo

| Módulo | Boss Name | Mecánica Especial |
|--------|-----------|-------------------|
| Horizontal | The Line Walker | Navegar línea compleja sin j/k |
| Vertical | The Code Tower | Archivo de 50 líneas, llegar a puntos específicos |
| Text Objects | The Bracket Demon | Código anidado `{[({})]}`, cambiar contenidos |
| Change & Repeat | The Clone Army | 10 ocurrencias, reemplazar selectivamente con cgn |
| Sustitución | The Transformer | Transformaciones complejas con rangos y flags |
| Regex | The Pattern Master | Encontrar patterns complejos en código real |
| Macros | The Automaton | Grabar macro y aplicar en múltiples líneas |
| Editing & Undo | The Historian | Editar, deshacer y rehacer hasta dejar el buffer pedido |
| Registers & Indentation | The Archivist | Copiar, pegar y conservar el registro correcto |

### Mecánicas de Boss

| Mecánica | Descripción |
|----------|-------------|
| ❤️ Vidas | 3 errores y perdés (retry desde el inicio) |
| ⏱️ Timer | El mínimo que la forma del paso necesita (ver abajo), no un número elegido a mano |
| Cadena | 5 pasos seguidos, todo conectado |
| Combo | Respuestas correctas seguidas suben el multiplicador (x2, x3, x4) en los tres modos, y el header lo muestra |
| Tiempo | La pelea mide su duración y la pantalla de resultado muestra la pelea y el mejor del módulo |

#### El reloj del jefe es una regla, no un número

`BossStepRecommendedSeconds` (`bossclock.go`) es el único dueño de "cuánto tiempo
necesita este paso":

```text
4 s de base  +  misión / 20 caracteres por segundo  +  1 s por carácter de la respuesta
```

con tope de 90 s. El `TimeLimit` de cada paso tiene que ser al menos ese valor:
el mismo tipo de paso recibía 5 s en un módulo y 30 s en otro, y un paso con una
misión de 254 caracteres tenía 15 s para leerla y contestar. Una pelea que se
pierde por el reloj y no por el jugador es lo contrario de lo que un jefe busca.
`BonusTime` —los segundos extra que un paso ganado le regala al siguiente— vive
entre 15 y 60 s: una pelea que no da nada hace que ganar no valga nada, y una que
da minutos vuelve decorativo su reloj. Los tres valores los vigila
`corpus_contract_test.go` sobre el contenido real.

---

## Estructura de Datos

### Exercise

```go
type Exercise struct {
    ID             string       // "horizontal_001"
    Module         ModuleID     // "horizontal", "vertical", "textobjects", ...
    Level          int          // 1-10, y es lo que el ejercicio paga (ver abajo)
    Type           ExerciseType // lesson, practice, boss
    Code           []string     // Líneas de código a mostrar
    CursorPos      Position     // Dónde está el cursor inicialmente
    Mission        string       // Qué debe lograr el jugador
    Solutions      []string     // ["w", "W", "fe"] - todas las válidas
    Optimal        string       // "w" - la mejor/más corta
    Hint           string       // Pista que aparece después del timeout
    Explanation    string       // Explicación post-respuesta
    TimeoutSecs    int          // Segundos antes de mostrar la solución
    Points         int          // Puntos base por completar (piso: ver PointsForLevel)
    BufferVerified bool         // Opta al juez de buffer (ver "Cómo se juzga un ejercicio")
}

type Position struct {
    Line int
    Col  int
}
```

Los dos campos que deciden el comportamiento son `Module` (el módulo al que
pertenece el ejercicio y que fija cuándo se desbloquea) y `BufferVerified` (el
juez que lo comprueba). `Solutions` es la lista de respuestas aceptadas y
funciona como verdad de referencia: una respuesta listada se acepta aunque el
motor no pueda reproducirla todavía. `Optimal` es la que el motor debe poder
ejecutar y la que se muestra al jugador.

#### El nivel es un contrato

`Level` va de 1 a 10 y lo vigila `TestCorpusLevelsAreInRange` sobre todo el
corpus. No es un comentario: es lo que paga un acierto. `PointsForLevel` da
`10 + 5 * (nivel - 1)`, o sea de 10 a 55 puntos, y `ExercisePoints` devuelve el
mayor entre ese valor y el `Points` escrito a mano, así que una recompensa
autoral sobrevive y un ejercicio difícil nunca puede pagar menos que uno fácil.
Antes de esta regla `Points` era el único dueño del puntaje y se estancaba: un
ejercicio de nivel 6 pagaba exactamente lo mismo que uno de nivel 3.

El nivel también sesga la práctica. `MasteredLevelFloor` es el nivel más alto del
módulo que está completo, y el pozo suma `PracticeFrontierWeight` a los
ejercicios un nivel por encima de ese piso, `PracticeBeyondFrontierWeight` a los
más lejanos, y nada a los que ya quedaron atrás: la práctica es una progresión y
no una lotería.

#### Qué enuncia la misión y qué agrega la pista

`Mission` y `Hint` se reparten el texto, y cada uno tiene un trabajo distinto:

- `Mission` enuncia el objetivo. Dice dónde debe quedar el cursor, qué debe leer
  el buffer o, en los ejercicios cuya respuesta es una secuencia exacta de teclas
  que el entrenador no ejecuta, el comando que se pide tipear. Es la frase con la
  que puede actuar quien ya conoce el comando.
- `Hint` enuncia el mecanismo que la misión no da: dónde cae el comando, qué
  incluye o deja afuera, qué cuenta, bandera o rango toma, cómo se compara con su
  comando hermano más cercano, o qué se sigue de él. Es la línea que el jugador
  pide con Tab (y que aparece sola tras `TimeoutSecs`), así que es la que enseña.

Una pista tiene que agregar al menos una de esas cosas. La regla nace de un
defecto reportado por un jugador: la misión decía "using w" y la pista decía "w
moves to the start of the next word", así que pedir la pista costaba una tecla y
no enseñaba nada, la misma frase dos veces.

Una pista que solo puede repetir su misión no es una pista: o se escribe el
mecanismo que la misión deja implícito, o se quita la pista y el ejercicio queda
con su misión y su `Explanation`. Quitar una pista ya es una operación de un solo
paquete: `trainerHintLabel` (en `update.go`) es el único lugar que arma la
etiqueta y devuelve cadena vacía cuando no hay pista, así que una pista ausente
no dibuja ninguna etiqueta. La misma función sirve a las pantallas de lección,
práctica y jefe, y a la revelación automática tras `TimeoutSecs`.

La tecla Tab revela la pista en la pantalla de jefe con la misma etiqueta que en
la de ejercicio, pero a través de `trainer.BossStepNudge`: la pista del paso
cuando existe, y si no los primeros caracteres de su respuesta. El respaldo
existe porque Tab estaba muerta en los 35 pasos de jefe que no traían pista, y
nunca imprime la respuesta completa: una respuesta de dos teclas muestra la
primera, y una de una sola tecla no muestra nada, porque su primera tecla es su
solución. Los 45 pasos traen pista escrita —`TestCorpusBossStepsCarryAHint` lo
exige y `TestCorpusHintsDoNotQuoteTheAnswer` impide que una pista cite su propia
respuesta—, así que el respaldo es la regla para el contenido que venga. La
revelación automática por `TimeoutSecs` sigue siendo solo de los ejercicios,
porque el reloj del jefe cuesta una vida y es otro mecanismo.

`TestShippedHintsAddWhatTheirMissionDoesNot` (en `exercises_test.go`) recorre
todo el corpus y exige una de esas adiciones en cada pista; `hintEchoRewrites`
fija los ecos exactos que esta pasada retiró, para que una edición posterior no
los reponga sin que un test lo note.

### Boss Exercise

```go
type BossExercise struct {
    ID          string
    Module      string
    Name        string       // "The Line Walker"
    Lives       int          // 3
    Steps       []BossStep   // Cadena de misiones
    BonusTime   int          // Segundos extra para el paso que sigue a uno ganado (15-60)
}

type BossStep struct {
    Exercise    Exercise
    TimeLimit   int          // Segundos para este paso: >= BossStepRecommendedSeconds
}
```

### User Stats

```go
type UserStats struct {
    TotalScore      int
    CurrentStreak   int
    BestStreak      int
    TotalTime       time.Duration
    ModuleProgress  map[string]*ModuleProgress
    BossesDefeated  []string
    LastPlayed      time.Time
}

type ModuleProgress struct {
    // Lecciones
    LessonsCompleted  int
    LessonsTotal      int
    
    // Práctica  
    PracticeAccuracy  float64  // 0.0 - 1.0, sobre toda la historia
    PracticeAttempts  int
    PracticeCorrect   int
    RecentResults     []bool   // Últimos BossGateWindow (20) aciertos: es lo que lee la puerta del jefe
    
    // Boss
    BossDefeated      bool
    BossBestTime      time.Duration
    BossAttempts      int
    
    // Spaced Repetition
    WeakExercises     []string  // IDs de ejercicios que más falla
    LastPracticed     time.Time

    // Por ejercicio (ModuleProgress.ExerciseStats): Mastered, LastAttempted y los
    // contadores. ExerciseStats.IsReviewDue compara LastAttempted con
    // MasteryReviewInterval (12 h): lo dominado vuelve a la ronda de repaso en vez
    // de quedarse fuera para siempre.
}
```

### Archivo de Stats

Guardar en `~/.config/dotfiles-trainer/stats.json`

Tres reglas de `stats.go`, cada una con su test (`stats_atomic_test.go`):

- **Escritura atómica.** `SaveStats` serializa primero, escribe en un archivo
temporal creado en el mismo directorio del destino —un rename entre sistemas de
archivos es una copia, y por eso la ubicación importa— y lo renombra sobre el
perfil. Un corte a mitad de escritura no puede dejar un perfil truncado donde
había uno legible, y toda ruta de fallo borra su propio temporal.
- **Versión de esquema.** El JSON lleva `schemaVersion` (`StatsSchemaVersion`). Un
perfil escrito antes de que el campo existiera carga igual (versión 0 es este
formato), y uno escrito por un entrenador más nuevo se preserva en vez de
rebajarse: esta build no ve los campos que no conoce y volver a guardarlo los
borraría.
- **Nada se pierde en silencio.** `LoadStatsReport` distingue el caso normal (no
hay perfil todavía: es la primera partida, no se reporta nada) de un perfil que
no se puede parsear. En ese caso renombra el archivo a un hermano
`stats.json.corrupt-<epoch>` o `stats.json.newer-<epoch>` y devuelve una frase que
nombra dónde quedó; `startTrainer` la muestra como mensaje del entrenador.
`LoadStats` sigue existiendo como envoltorio para quien solo describe un panel.

```json
{
  "schemaVersion": 1,
  "totalScore": 2340,
  "currentStreak": 7,
  "bestStreak": 23,
  "totalTimeSeconds": 8280,
  "lastPlayed": "2026-01-01T15:30:00Z",
  "bossesDefeated": ["horizontal", "vertical"],
  "modules": {
    "horizontal": {
      "lessonsCompleted": 15,
      "lessonsTotal": 15,
      "practiceAccuracy": 0.85,
      "practiceAttempts": 47,
      "practiceCorrect": 40,
      "recentResults": [true, true, false, true],
      "bossDefeated": true,
      "bossBestTimeSeconds": 28,
      "bossAttempts": 3,
      "weakExercises": ["horizontal_012", "horizontal_008"],
      "lastPracticed": "2026-01-01T15:30:00Z"
    }
  }
}
```

---

## Estructura de Archivos

```text
installer/internal/tui/
├── model.go             # Estado de la app, pantallas y opciones
├── update.go            # Manejo de teclas y mensajes
├── view.go              # Render de todas las pantallas, incluido el trainer
├── styles.go            # Paleta y estilos Lipgloss
└── trainer/
    ├── types.go            # Ejercicios, módulos, progreso y estadísticas
    ├── exercises.go        # Registro de módulos, lecciones y bosses
    ├── exercises_*.go      # Corpus de cada módulo (horizontal, editing, ...)
    ├── simulator.go        # Simulador de movimientos (juez de movimiento)
    ├── editor.go           # Motor de edición mutable (juez de buffer)
    ├── validation.go       # Decide qué juez usa cada respuesta
    ├── practice.go         # Selección ponderada de práctica
    ├── gamestate.go        # Sesión de lección, práctica y boss
    └── stats.go            # Persistencia de estadísticas en JSON
```

---

## Integración con TUI Existente

### 1. Agregar Screen en constants.go

```go
const (
    // ... screens existentes ...
    
    // Vim Trainer Screens
    ScreenVimTrainer        Screen = "vimtrainer"
    ScreenVimTrainerModule  Screen = "vimtrainer_module"
    ScreenVimTrainerLesson  Screen = "vimtrainer_lesson"
    ScreenVimTrainerPractice Screen = "vimtrainer_practice"
    ScreenVimTrainerBoss    Screen = "vimtrainer_boss"
    ScreenVimTrainerResult  Screen = "vimtrainer_result"
)
```

### 2. Agregar opción en menú principal

En `model.go`, agregar "🎮 Vim Mastery Trainer" como opción del menú principal.

### 3. Handler en update.go

Cuando se seleccione la opción del trainer, cambiar a `ScreenVimTrainer` y delegar al sub-model del trainer.

---

## Componentes Bubbletea

```go
import (
    "github.com/charmbracelet/bubbles/progress"   // Barras de progreso
    "github.com/charmbracelet/bubbles/timer"      // Countdown timer
    "github.com/charmbracelet/bubbles/textinput"  // Input del usuario
    "github.com/charmbracelet/bubbles/stopwatch"  // Medir tiempo de respuesta
    "github.com/charmbracelet/lipgloss"           // Estilos
)
```

---

## Comandos de Desarrollo

Ejecutar desde `installer/`:

| Comando | Descripción |
|---------|-------------|
| `go build -o dotfiles ./cmd/dotfiles` | Build del binario |
| `go test ./...` | Ejecutar todos los tests |
| `./dotfiles` | Ejecutar el installer |
| `go test ./internal/tui/trainer/... -v` | Tests específicos del trainer |

---

## Plan de Implementación (MVP)

### Fase 1: Estructura Base
- [ ] Crear estructura de archivos
- [ ] Model base del trainer con navegación
- [ ] Integración con menú principal
- [ ] Pantalla de selección de módulos

### Fase 2: Primer Módulo (Horizontal)
- [ ] 15 ejercicios de lección (guiados)
- [ ] Sistema de timer + input
- [ ] Validación de respuestas
- [ ] Pantalla de resultado con explicación
- [ ] 30 ejercicios de práctica (aleatorios)

### Fase 3: Boss Fight
- [ ] UI de boss con vidas y HP
- [ ] Cadena de 5 misiones
- [ ] Sistema de combos
- [ ] Pantalla de victoria/derrota

### Fase 4: Persistencia
- [ ] Guardar/cargar stats en JSON
- [ ] Tracking de progreso por módulo
- [ ] Spaced repetition básico

### Fase 5: Módulos Adicionales
- [ ] Vertical Motions
- [ ] Text Objects
- [ ] Change & Repeat (cgn)
- [ ] Sustitución
- [ ] Regex & Vimgrep
- [ ] Macros

---

## Estilo de Código

| Aspecto | Guideline |
|---------|-----------|
| Patterns | Seguir patterns existentes en el TUI (`installer.go`, `model.go`) |
| Estilos | Usar Lipgloss (referencia: `styles.go`) |
| Testing | Tests para ejercicios, validación y scoring |
| Commits | Conventional commits para cada feature |