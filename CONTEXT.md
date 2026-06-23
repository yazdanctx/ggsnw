# ggsnw — Domain Glossary

## Core concepts

**Shortname** — a partial filename fragment (e.g. `auth`, `user`) that the tool expands into full word candidates.

**Expansion** — the act of resolving a shortname into candidate words via a chosen Source.

**Source** — the mechanism used for expansion. Two kinds:
  - `github`: searches GitHub Code Search API for filenames containing the shortname, extracts matching path segments.
  - `ai`: prompts OpenAI to generate plausible full words that contain the shortname as a substring.

**Word** — a single candidate string produced by Expansion. Words are deduplicated and accumulated.

**Wordlist** — the accumulated, deduplicated set of Words produced during a Session.

**Session** — a persistent TUI session with a main menu. A Session has one Source (chosen at start) and a mutable Wordlist. The user can load Shortnames from a file, expand them, export the Wordlist, and navigate between views.

**Config** — persisted credentials (GitHub token, OpenAI API key) stored in `~/.config/ggsnw/config.json`.
