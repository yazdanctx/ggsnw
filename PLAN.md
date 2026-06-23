# ggsnw Rewrite Plan

## Domain

See `CONTEXT.md` for the full glossary. Key modules map to domain concepts 1:1.

## Package layout

```
src/ggsnw/
├── main.go               ← entry point, starts Bubble Tea program
├── config/
│   └── config.go         ← load/save/prompt for credentials (GitHub token, OpenAI key)
├── source/
│   ├── source.go         ← Source interface: Expand(context.Context, string) -> ([]string, error)
│   ├── github.go         ← GitHub Code Search API expansion
│   └── openai.go         ← OpenAI expansion via chat completion
├── wordlist/
│   └── wordlist.go       ← deduplicated set of Words, Load/Export methods
├── tui/
│   └── tui.go            ← Bubble Tea model, all views, update loop
├── go.mod
└── CONTEXT.md
```

## TUI Screens (Bubble Tea)

1. **Config check (in main.go)** — before launching TUI, check if required credentials exist for each source; if missing, prompt inline. Done before the TUI starts so the user isn't thrown around mid-session.

2. **Mode Select** — pick `github` or `ai`. Arrow keys + enter. Sets the session's Source.

3. **Main Menu** — options:
   - [1] **Expand Shortname** — text input for one shortname, shows results inline, returns to menu
   - [2] **Load from file** — enter a file path, reads shortnames line-by-line, expands all, shows results
   - [3] **View Wordlist** — scrollable list of accumulated words
   - [4] **Export Wordlist** — enter output file path, writes sorted unique words
   - [5] **Clear Wordlist** — confirm, then reset
   - [6] **Quit** — exit

4. **Back navigation** — every sub-screen has a back option (Esc or 'b') to return to the Main Menu without losing accumulated state.

## State flow

```
Start → config check → Mode Select → Main Menu ←→ sub-screens
                                       ↑______________|
```

The Wordlist persists across the session. Only Quit or Clear resets it.

## Source interface

```go
type Source interface {
    Name() string
    Expand(ctx context.Context, shortname string) ([]string, error)
}
```

Both `githubSource` and `openaiSource` implement it. The session holds one `Source` chosen at startup.

## Config persistence

Same as now: `~/.config/ggsnw/config.json`. On launch:
- If `github` mode is chosen and no token is found, prompt before entering TUI.
- Same for `ai` mode and no OpenAI key.

## What's carried over from main.go

- GitHub search pagination + rate-limit handling
- AI guessing with configurable guess count
- Credential prompting and persistence
- Dedup and sort of wordlist
- File-based input/output

## What's dropped

- `flag` package entirely
- CLI-only usage paths
