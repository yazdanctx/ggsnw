# ggsnw

Shortname fragment expansion tool. Takes a partial filename (e.g. `auth`, `admin`) and generates a wordlist of full filename candidates by searching GitHub Code Search or asking OpenAI.

## Install

```
go install github.com/yazdanctx/ggsnw@latest
```

## Usage

```
ggsnw
```

Launches an interactive TUI. Pick a source (GitHub or AI), then:

- **1** — Expand a single shortname
- **2** — Load shortnames from a file
- **3** — View current wordlist count
- **4** — Export wordlist to a file
- **5** — Clear wordlist
- **6** — Quit

## Tokens

**GitHub token:** Needed for GitHub Code Search (30 req/min with token, 10 without). Get one at https://github.com/settings/tokens — no scopes needed for public repos.

**OpenAI key:** Needed for AI mode (gpt-3.5-turbo). Get one at https://platform.openai.com/api-keys.

Tokens are stored at `~/.config/ggsnw/config.json` and prompted on first use if missing.
