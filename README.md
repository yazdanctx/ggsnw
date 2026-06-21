# ggsnw

Recon tool that takes IIS shortnames (e.g. `ADMIN~1`, `WEB~1.CON`) and expands them into a wordlist using either GitHub Code Search (real filenames) or OpenAI (AI-generated guesses).

## Install

```
go install github.com/yazdanctx/ggsnw@latest
```

## Tokens

### GitHub token (for `--mode github`, default)
Needed for GitHub Code Search API. Without one, you get 10 req/min; with a token, 30 req/min.
Get one at https://github.com/settings/tokens (no scopes needed for public repos).

### OpenAI key (for `--mode ai`)
Needed for AI-powered name guessing using gpt-3.5-turbo.
Get one at https://platform.openai.com/api-keys

Set either with `--token` / `--key` or let the tool prompt you on first use. Stored at `~/.config/ggsnw/config.json`.

## Usage

```
# GitHub mode (default) — finds real filenames
ggsnw --token ghp_xxx
ggsnw ADMIN~1
ggsnw -f shortnames.txt -o wordlist.txt

# AI mode — generates plausible name guesses
ggsnw --key sk-xxx --mode ai ADMIN~1
ggsnw --mode ai -f shortnames.txt --guesses 15
```

GitHub mode is rate-limited to 30 requests/minute; the tool pauses automatically.
