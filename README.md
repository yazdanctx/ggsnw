# ggsnw

Recon tool that takes IIS shortnames (e.g. `ADMIN~1`, `WEB~1.CON`) and extracts real filenames from GitHub Code Search — useful for expanding directory listing findings.

## Install

```
go install github.com/yazdanctx/ggsnw@latest
```

## Token

ggsnw needs a GitHub personal access token to search code. Without one, GitHub limits you to 10 unauthenticated requests/minute; with a token you get 30/min.

Get a token at https://github.com/settings/tokens — no scopes are needed for public repo searches.

On first run, ggsnw will prompt you to enter one. It's stored at `~/.config/ggsnw/config.json` for subsequent runs.

## Usage

```
# set token (once)
ggsnw --token ghp_xxxxxxxxxxxx

# single shortname
ggsnw ADMIN~1

# batch from file
ggsnw -f shortnames.txt -o wordlist.txt
```

Rate-limited to 30 requests/minute (GitHub API limit for authenticated code search); the tool automatically pauses when near the limit.
