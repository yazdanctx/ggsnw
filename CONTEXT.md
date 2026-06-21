# ggsnw — Domain Glossary

## Core Concepts

### Shortname
A truncated 8.3-style IIS shortname (e.g. `ADMIN~1`, `WEB~1.CON`). The input to the tool.

### Real name
The full filename extracted from a GitHub Code Search result whose path contains the shortname (case-insensitive match against any path segment). The output of the tool.

### Wordlist
The set of unique real names collected across all searched shortnames, deduplicated.

## Tool Behaviour

### Input modes
- Single shortname: positional argument (`ggsnw ADMIN~1`)
- Shortname list file: `-f shortnames.txt` (one per line)

### Output format
- Live during search: `\t[x] {realname}` printed per previously-unseen match
- No-match per shortname: `\t[ ] {shortname} not found`
- Final summary: `-----` border, one realname per line, `-----` border
- Optional file output: `-o wordlist.txt`

### Token persistence
- First run: `ggsnw --token ghp_xxx` writes token
- Stored at: `~/.config/ggsnw/config.json`
- Subsequent runs: token read automatically

### API behaviour
- Uses GitHub Code Search API (authenticated)
- Full pagination per query (up to 1000 results max, GitHub-enforced limit)
- Rate-limit aware: respects `X-RateLimit-Remaining`, sleeps when near empty
- No parallelism; sequential per-shortname

## Limits

| Limit | Value |
|---|---|
| Max results per query | 1000 (10 pages × 100) |
| Rate limit (authenticated) | 30 req/min |
| Match mode | Case-insensitive, any path segment |
