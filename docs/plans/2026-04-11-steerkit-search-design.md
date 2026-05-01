# SteerKit Deep Search Integration

Adds semantic search to cc-tui via the SteerKit daemon API. Lets users find sessions by searching conversation content and episodes, then jump directly to the matching session in the tree.

## Decisions

- `s` key for deep search; `/` stays as fast local filter
- Results limited to conversations + episodes (both carry `session_id` directly)
- Flat ranked list replaces the tree temporarily
- Enter on a result navigates to that session in the tree
- SteerKit availability probed at startup via `GET /health`; `s` key only enabled when reachable
- All SteerKit communication is HTTP from the TUI client; the cc-tui daemon is not involved

## Not in scope (initial version)

- Knowledge entry results (require lazy `GET /entry/{id}/source` to resolve session)
- Searching from within preview mode
- Caching or pagination of results

## Data Flow

```
User presses 's'
    |
App enters search mode -> show text input at footer
    |
User types query, presses Enter
    |
App sends HTTP GET to http://127.0.0.1:7419/recall?q={query}&limit=20
    |
SteerKit returns ranked results (exchanges + episodes with session_id, score, summary)
    |
App enters search results mode -> tree replaced by flat ranked list
    |
User navigates with up/down, presses Enter on a result
    |
App exits search mode -> finds session by UUID in a.groups
    |
Expands the parent project group, sets cursor to that session
    |
Normal tree view resumes with the session highlighted
```

### Esc behavior

- In the text input: cancel search, return to tree
- In the results list: cancel search, return to tree

### Edge cases

- No results: show "No results for '{query}'" in the list area
- Session ID from SteerKit not found in cc-tui's cache: show result dimmed with "(session not in cache)" — the session may be older than the top-25 project limit

## Result Row Format

```
 0.87  C  "How do I handle duplicate inserts..."   my-project   Apr 10
 0.72  E  Fixed sqlite-vec duplicate key handling   steer-kit    Apr 09
```

Score (dimmed), type indicator (C=conversation, E=episode), snippet/summary, project name, relative date.

## Implementation Changes

### New files

**`tui/search.go`** — SearchState struct and rendering logic.

```go
type SearchResult struct {
    SourceType string    // "exchange" or "episode"
    SessionID  string
    Score      float64
    Summary    string
    Project    string
    Timestamp  time.Time
}

type SearchState struct {
    input     textinput.Model
    results   []SearchResult
    cursor    int
    offset    int
    querying  bool
    noResults bool
    width, height int
}
```

### Modified files

**`tui/app.go`**
- Add fields: `steerKitAvailable bool`, `showSearch bool`, `search SearchState`
- Init: fire `checkSteerKitCmd` that does `GET /health` -> returns `steerKitAvailableMsg{bool}`
- Update: when `s` pressed and `steerKitAvailable`, enter search mode. Route keys to `search.Update()` when `showSearch` is true (same overlay interception pattern as preview/filter).
- View: when `showSearch`, render `search.View()` instead of `tree.View()`

**`tui/keymap.go`**
- Add `Search key.Binding` bound to `s`

**`tui/banner.go`**
- When `steerKitAvailable`, show a subtle indicator next to the status

### Jump-to-session logic

When the user selects a search result:

1. Receive `searchSelectMsg{sessionID}` from search state
2. Set `showSearch = false`
3. Iterate `a.groups` to find the `ProjectGroup` containing a session with matching UUID
4. If not found: set `a.err` with a brief message, return to tree as-is
5. If found: set `a.expandState[group.Project] = true`
6. Rebuild tree: `a.tree.SetGroups(a.applyFilter(a.groups), a.expandState)`
7. Walk `a.tree.visible` to find the node where `node.Session.ID == sessionID`
8. Set cursor and adjust scroll offset to center it

**New method on `TreeModel`:**

```go
func (m *TreeModel) ScrollToSession(sessionID string) bool
```

Scans `m.visible`, sets cursor + scroll offset. Returns false if not found (node may be filtered out by active `/` filter — clear the filter first in that case).

## SteerKit API Endpoints Used

| Endpoint | Purpose |
|----------|---------|
| `GET /health` | Startup probe — enable/disable search feature |
| `GET /recall?q={query}&limit=20` | Semantic search across conversations + episodes |

SteerKit daemon runs on `http://127.0.0.1:7419` (configurable via `STEERKIT_DAEMON_PORT`).
