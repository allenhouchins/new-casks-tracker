# New Casks Tracker

Tracks newly added [Homebrew casks](https://formulae.brew.sh/cask/) and
publishes them to a static GitHub Pages site, newest first.

Every 6 hours a GitHub Actions workflow:

1. Fetches the full cask index from `https://formulae.brew.sh/api/cask.json`.
2. Diffs it against the previously seen set of cask tokens
   (`data/known_casks.json`).
3. Appends any newly discovered casks — with a UTC detection timestamp — to
   the append-only history log (`data/new_casks_log.json`).
4. Regenerates the static site in `docs/` (`index.html` shows the 200 most
   recent, `history.html` shows everything, `feed.xml` is an RSS 2.0 feed
   of the 100 most recent).
5. Commits and pushes the updated state and site back to the repo.

Everything is standard-library Go — no external dependencies, no JavaScript
on the generated pages.

## Repo layout

| Path | Purpose |
| --- | --- |
| `main.go` | Orchestrates fetch → diff → update state → regenerate site |
| `internal/casks/` | Fetching/parsing the cask index and state persistence |
| `internal/site/` | Static site rendering |
| `templates/index.html.tmpl` | Go `html/template` for both pages |
| `templates/style.css` | Stylesheet, copied into `docs/` on each run |
| `data/known_casks.json` | Full set of cask tokens seen so far |
| `data/new_casks_log.json` | Append-only history of new casks with timestamps |
| `docs/` | Generated GitHub Pages site |
| `.github/workflows/check-casks.yml` | Scheduled workflow (every 6 h + manual) |

## Running locally

Requires Go 1.23+.

```bash
go run .
```

Flags (all optional):

- `-api-url` — cask index URL, or a **local JSON file path** for offline
  testing (default: the Homebrew API).
- `-data-dir` — where state files live (default: `data`).
- `-docs-dir` — where the site is generated (default: `docs`).
- `-site-url` — absolute URL the site is published at, used as the channel
  link in the RSS feed (default: the GitHub Pages URL of this repo).

Preview the generated site by opening `docs/index.html` in a browser.

## First baseline run

On the very first run `data/known_casks.json` doesn't exist yet, so the
program records **all** current casks as the baseline and flags nothing as
new (otherwise the first run would dump ~7,000 "new" casks at once). New
casks are detected from the second run onward.

To establish the baseline, either:

- run `go run .` locally and commit the resulting `data/` and `docs/`
  directories, or
- push the repo and trigger the workflow manually: **Actions → Check for
  new casks → Run workflow** (the workflow commits the baseline for you).

## Enabling GitHub Pages

1. Push the repo to GitHub (default branch `main`).
2. Go to **Settings → Pages**.
3. Under **Build and deployment**, set **Source** to "Deploy from a branch",
   then choose branch `main` and folder `/docs`.
4. Save. The site will be published at
   `https://<user>.github.io/<repo>/` a minute or two after the next push.

The workflow needs no extra secrets — the default `GITHUB_TOKEN` with
`permissions: contents: write` (already set in the workflow file) is enough
to push. If pushes are rejected, check **Settings → Actions → General →
Workflow permissions** and allow read/write.

## Failure behavior

- If the fetch or parse fails (network error, non-200 response, malformed
  JSON), the program exits non-zero **before touching any state files**, so
  the Actions run shows as failed and nothing is lost or corrupted.
- State files are written atomically (temp file + rename), so a crash
  mid-write can't truncate them.
- Cask entries missing a `token` are skipped rather than crashing the run;
  missing names fall back to the token, and missing descriptions/homepages
  are simply omitted from the page.
