# ggsnw

Recon tool that takes IIS shortnames (e.g. `ADMIN~1`, `WEB~1.CON`) and extracts real filenames from GitHub Code Search — useful for expanding directory listing findings.

## Install

```
go install github.com/yazdanctx/ggsnw@latest
```

## Usage

```
# set token (once)
ggsnw --token ghp_xxxxxxxxxxxx

# single shortname
ggsnw ADMIN~1

# batch from file
ggsnw -f shortnames.txt -o wordlist.txt

# suppress banner
ggsnw -silent ADMIN~1
```

Token is stored at `~/.config/ggsnw/config.json`.

Rate-limited to 30 requests/minute (GitHub API limit for authenticated code search); the tool automatically pauses when near the limit.
