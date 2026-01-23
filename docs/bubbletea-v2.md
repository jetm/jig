# Bubbletea v2 — Critical

Claude's training data is v1 and will produce wrong code. All code uses v2 (stable 2026-02-23).

**Import paths — always use v2:**
```go
import (
    tea "charm.land/bubbletea/v2"    // NOT github.com/charmbracelet/bubbletea
    "charm.land/bubbles/v2/list"     // NOT github.com/charmbracelet/bubbles/*
    "charm.land/bubbles/v2/viewport"
    "charm.land/lipgloss/v2"         // NOT github.com/charmbracelet/lipgloss
)
```

**`View()` returns `tea.View`, not `string`** (root `app.Model` only):
```go
func (a *Model) View() tea.View {
    v := a.Active().View() // child commands return string
    v.AltScreen = true
    return v
}
```

**`tea.KeyPressMsg` not `tea.KeyMsg`:**
```go
case tea.KeyPressMsg:
    switch msg.String() {
    case "enter": …
    case "q": …
    case "space": …      // was " " in v1
    case "ctrl+c": …     // was tea.KeyCtrlC in v1
    }
```

**Bubbles v2 — width/height are methods, not fields:**
```go
m.list.SetWidth(40)          // not m.list.Width = 40
m.viewport.SetHeight(20)     // not m.viewport.Height = 20
w := m.list.Width()          // getter is a method too
```

**AltScreen, mouse, keyboard:** declarative on `tea.View`, not program options. Do not pass `tea.WithAltScreen()` to `tea.NewProgram()`.
