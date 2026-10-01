---
name: dotfiles-bubbletea
description: >
  Bubbletea TUI patterns for dotfiles installer.
  Trigger: When editing Go files in installer/internal/tui/, working on TUI screens, or adding new UI features.
license: Apache-2.0
metadata:
  author: albersg
  version: "1.0"
---

## When to Use

Use this skill when:
- Adding new screens to the TUI installer
- Handling keyboard input or navigation
- Creating new UI components with Lipgloss
- Working on screen transitions or state management

---

## Critical Patterns

### Pattern 1: Screen Constants in model.go

All screens MUST be defined as `Screen` constants in `model.go`:

```go
type Screen int

const (
    ScreenWelcome Screen = iota
    ScreenMainMenu
    ScreenOSSelect
    // ... new screens go here
    ScreenNewFeature      // Add new screen
    ScreenNewFeatureCat   // Add category screen if needed
)
```

### Pattern 2: Model Struct Holds All State

The `Model` struct in `model.go` holds ALL application state:

```go
type Model struct {
    Screen      Screen
    PrevScreen  Screen      // For back navigation
    Width       int
    Height      int
    Cursor      int
    // Add new state here
    NewFeatureData    []SomeType
    NewFeatureScroll  int
}
```

### Pattern 3: Update Pattern with Type Switch

All input handling goes through `Update()` with a type switch:

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        return m.handleKeyPress(msg)
    case tea.WindowSizeMsg:
        m.Width = msg.Width
        m.Height = msg.Height
        return m, nil
    case customMsg:
        // Handle custom messages
        return m, nil
    }
    return m, nil
}
```

### Pattern 4: Key Handlers Return (Model, Cmd)

Separate handler per screen, always return `(tea.Model, tea.Cmd)`:

```go
func (m Model) handleNewFeatureKeys(key string) (tea.Model, tea.Cmd) {
    options := m.GetCurrentOptions()

    switch key {
    case "up", "k":
        if m.Cursor > 0 {
            m.Cursor--
            // Skip separator
            if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor > 0 {
                m.Cursor--
            }
        }
    case "down", "j":
        if m.Cursor < len(options)-1 {
            m.Cursor++
            if strings.HasPrefix(options[m.Cursor], "───") && m.Cursor < len(options)-1 {
                m.Cursor++
            }
        }
    case "enter", " ":
        // Handle selection
        return m.handleNewFeatureSelection()
    case "esc":
        m.Screen = m.PrevScreen
        m.Cursor = 0
    }
    return m, nil
}
```

---

### Pattern 5: The invariants the guards enforce

Every one of these came out of a defect that reached a real terminal, so they are not style preferences.
Each has a guard that fails when it is broken, and the guards live in `teatest_test.go` and
`render_leak_test.go` unless the entry says otherwise.

1. **The frame's rows are reserved, not headroom.** Every screen renders inside the shared frame, and both
   frame guards measure it at the 80x24 floor and at wide sizes. **A new screen has to be added to the list
   those guards iterate** - otherwise it ships unmeasured, and nothing complains. `installerBodyRows` and the
   per-screen `*BodyFixed` constants are how a screen reserves the rows it needs; spending a row nobody
   reserved fails the guard.
2. **Colour never carries meaning on its own.** Words and glyphs do. A 16-colour or colourless terminal must
   lose nothing, and Termux is a supported terminal: no emoji with meaning, no state that only truecolour
   shows.
3. **The render path reads nothing.** No clock, no file, no environment, no terminal query while drawing:
   everything a screen shows arrives on the model, which is what lets a snapshot pin it. The gates
   (`--no-anim`, `--no-mouse`, `--no-sprite`, the `DOTFILES_*` switches) are read **once, when the model is
   built**, and the model carries the answer.
4. **One ladder, one renderer.** The companion is chosen by one ladder and drawn by one set of functions. A
   screen that composes its own rows - the trainer does - must call them rather than reimplement the art:
   two copies drift, and the drift is invisible until somebody notices the old creature on one screen.
5. **A tick may change only the rows a widget owns**, and at rest, with nothing happening, the view must be
   byte-identical from tick to tick so the renderer writes nothing at all.
6. **Escape sequences are retired.** A cell that stops being painted must reset the style it set: an active
   **background** paints every cell after it, to the end of the line and on through the rows below, which is
   how a sprite once became a bar of colour across the terminal.
7. **Snapshots are regenerated from the merged code and inspected, never hand-merged**, and one that moves
   for a legitimate reason is reported with its diff. Look at both the shape and the escapes: a shape-only
   check cannot see a leak, and an escape-only check cannot see art that moved.
8. **A number in prose comes from a test that prints it** - `TestCompanionCostHasTwoRegimes` for the
   companion's two cost regimes, `BenchmarkCompanionVolumeFrame` for what rendering costs,
   `TestNoRenderedLineLeavesAColourActive` for the leak. Prose decays silently; a figure with a test behind
   it can be re-derived.

---

## Decision Tree

```
Adding a new screen?
├── Define Screen constant in model.go
├── Add state fields to Model struct
├── Add handler in handleKeyPress switch
├── Create handle{Screen}Keys function in update.go
├── Add view case in view.go
└── Add title in GetScreenTitle()

Adding navigation to existing screen?
├── Use m.PrevScreen for back navigation
├── Reset m.Cursor = 0 on screen change
└── Save scroll position if scrollable

Adding scrollable content?
├── Add {Screen}Scroll int to Model
├── Calculate visibleItems from m.Height
├── Handle up/down for scroll position
└── Reset scroll on screen exit
```

---

## Code Examples

### Example 1: Adding Screen to handleKeyPress

```go
// In handleKeyPress switch statement:
case ScreenNewFeature:
    return m.handleNewFeatureKeys(key)

case ScreenNewFeatureCat:
    return m.handleNewFeatureCatKeys(key)
```

### Example 2: Screen Options Pattern

```go
func (m Model) GetCurrentOptions() []string {
    switch m.Screen {
    case ScreenNewFeature:
        categories := make([]string, len(m.NewFeatureData)+2)
        for i, item := range m.NewFeatureData {
            categories[i] = item.Name
        }
        categories[len(m.NewFeatureData)] = "─────────────"
        categories[len(m.NewFeatureData)+1] = "← Back"
        return categories
    // ...
    }
}
```

### Example 3: Scrollable View Pattern

```go
func (m Model) handleNewFeatureCatKeys(key string) (tea.Model, tea.Cmd) {
    data := m.NewFeatureData[m.SelectedNewFeature]

    visibleItems := m.Height - 9
    if visibleItems < 5 {
        visibleItems = 5
    }

    maxScroll := len(data.Items) - visibleItems
    if maxScroll < 0 {
        maxScroll = 0
    }

    switch key {
    case "up", "k":
        if m.NewFeatureScroll > 0 {
            m.NewFeatureScroll--
        }
    case "down", "j":
        if m.NewFeatureScroll < maxScroll {
            m.NewFeatureScroll++
        }
    case "esc", "q", "enter", " ":
        m.Screen = ScreenNewFeature
        m.NewFeatureScroll = 0
    }
    return m, nil
}
```

### Example 4: Custom Message Pattern

```go
// Define message type
type newFeatureLoadedMsg struct {
    data []SomeType
    err  error
}

// Send message from command
func loadNewFeatureCmd() tea.Cmd {
    return func() tea.Msg {
        data, err := loadData()
        return newFeatureLoadedMsg{data: data, err: err}
    }
}

// Handle in Update
case newFeatureLoadedMsg:
    if msg.err != nil {
        m.ErrorMsg = msg.err.Error()
        return m, nil
    }
    m.NewFeatureData = msg.data
    return m, nil
```

---

## Commands

```bash
cd installer && go build ./cmd/dotfiles  # Build installer
cd installer && go test ./internal/tui/...          # Run TUI tests
cd installer && go test -run TestNewFeature         # Run specific test
```

---

## Resources

- **Model**: See `installer/internal/tui/model.go` for state management
- **Update**: See `installer/internal/tui/update.go` for input handling
- **View**: See `installer/internal/tui/view.go` for rendering
- **Styles**: See `installer/internal/tui/styles.go` for Lipgloss styles
