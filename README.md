<p align="center">
  <img src="assets/logo.png" alt="Simili Logo" width="150">
</p>

# Simili Bot

<p align="center">
  <strong>AI-Powered GitHub Issue Intelligence.</strong>
</p>

<p align="center">
  <a href="https://github.com/similigh/simili-bot/actions"><img src="https://img.shields.io/github/actions/workflow/status/similigh/simili-bot/ci.yml?branch=main&style=flat-square" alt="Build Status"></a>
  <a href="https://github.com/similigh/simili-bot/releases"><img src="https://img.shields.io/github/v/release/similigh/simili-bot?style=flat-square" alt="Release"></a>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square" alt="License"></a>
  <a href="https://github.com/similigh/simili-bot"><img src="https://img.shields.io/github/stars/similigh/simili-bot?style=flat-square" alt="Stars"></a>
</p>

Automatically detect duplicate issues, find similar issues with semantic search, and intelligently route issues across repositories.

[![Star History Chart](https://api.star-history.com/svg?repos=similigh/simili-bot&type=date&legend=top-left)](https://www.star-history.com/#similigh/simili-bot&type=date&legend=top-left)

---

## Features

- **Semantic Duplicate Detection** — Find related issues using AI-powered embeddings, not just keyword matching.
- **Cross-Repository Search** — Search for similar issues across your organization.
- **Intelligent Routing** — Automatically transfer issues to the correct repository based on content.
- **Smart Triage** — AI-powered labeling and quality assessment.
- **Modular Pipeline** — Customize workflows with plug-and-play steps.
- **Multi-Repo Support** — Central configuration with per-repo overrides.

## Architecture

Simili uses a **"Lego with Blueprints"** architecture:

- **Lego Blocks**: Independent, reusable pipeline steps (Gatekeeper, Similarity, Triage, etc.).
- **Blueprints**: Pre-defined workflows for common use cases.
- **State Branch**: Git-based state management using an orphan branch (no comment scanning).

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ Gatekeeper  │───▶│  Similarity │───▶│   Triage    │───▶│   Action    │
│   Check     │    │   Search    │    │  Analysis   │    │  Executor   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

## Quick Start
Simili-Bot supports both **Single-Repository** and **Organization-wide** setups.

### Setup Guides

| Guide | Description |
|-------|-------------|
| [Single Repo Setup](DOCS/single-repo-setup.md) | Instructions for setting up Simili-Bot on a standalone repository. |
| [Organization Setup](DOCS/multi-repo-org-setup.md) | Best practices for deploying across an organization using Reusable Workflows. |

### AI Provider Configuration

Simili supports three providers: **GitHub Models** (zero-config), **Gemini**, and **OpenAI**.

#### GitHub Models — zero external key required

`GITHUB_TOKEN` is already present on every Actions runner. When no other AI key is configured, Simili automatically uses the [GitHub Models](https://docs.github.com/en/github-models/quickstart) inference endpoint with it — no extra secrets needed:

```yaml
# .github/simili.yaml — minimum zero-config setup
embedding:
  provider: github_models
  model: text-embedding-3-small

llm:
  provider: github_models
  model: gpt-4o-mini
```

Or omit the provider entirely — Simili falls back to `github_models` automatically when `GITHUB_TOKEN` is the only credential available.

> **Rate limits:** GitHub Models free tier allows 15 RPM / 150k tokens per day. Sufficient for active repos (50-200 issues/day). For bulk backfill with `simili index`, the built-in retry/backoff handles rate limiting automatically.

#### Gemini / OpenAI — bring your own key

- Set `GEMINI_API_KEY` or `OPENAI_API_KEY` as a repository secret
- If both are set, Gemini takes precedence
- The config `provider` field can explicitly pin the provider

Provider resolution order:
1. Explicit `provider: github_models` in config
2. `GEMINI_API_KEY` environment variable
3. `OPENAI_API_KEY` environment variable
4. Config `api_key` (provider inferred from key prefix)
5. Zero-config fallback — `GITHUB_TOKEN` with GitHub Models

Default models:

| Provider | LLM | Embeddings | Embedding dims |
|---|---|---|---|
| `github_models` | `gpt-4o-mini` | `text-embedding-3-small` | 1536 |
| `gemini` | `gemini-2.0-flash-lite` | `gemini-embedding-001` | 3072 |
| `openai` | `gpt-5.2` | `text-embedding-3-small` | 1536 |

If you override `embedding.model`, keep `embedding.dimensions` aligned:

- `gemini-embedding-001` -> `3072`
- `text-embedding-3-small` -> `1536`
- `text-embedding-3-large` -> `3072`

## Examples

We provide copy-pasteable examples to get you started quickly:

- **[Multi-Repo Examples](DOCS/examples/multi-repo)**: Includes shared workflow, caller workflow, and central config.
- **[Single-Repo Examples](DOCS/examples/single-repo)**: Standard workflow and configuration.

## Available Workflows

You can specify a `workflow` in your `simili.yaml` or define custom steps.

| Preset | Description |
|--------|-------------|
| `issue-triage` | Full pipeline: similarity search, duplicate check, triage analysis, and action execution. |
| `similarity-only` | Runs similarity search only. Useful for "Find Similar Issues" features without auto-triage. |
| `index-only` | Indexes issues to the vector database without providing feedback. |

## CLI Commands

Simili provides a powerful CLI for local development, testing, and batch operations.

### `simili index`

Bulk index issues from a GitHub repository into the vector database.

```bash
simili index --repo owner/repo --workers 5
```

**Flags:**
- `--repo` (required): Target repository (owner/name)
- `--since`: Start from issue number or timestamp
- `--workers`: Number of concurrent workers (default: 5)
- `--token`: GitHub token override (defaults to `GITHUB_TOKEN` env var)
- `--include-prs`: Index pull requests in addition to issues (default: true)
- `--dry-run`: Simulate without writing to database

**Error handling:** If any worker encounters an unrecoverable error (including a panic), the command exits with a non-zero status code and logs the failure. All in-flight workers are cancelled cleanly before exit — the process will not hang.

### `simili process`

Process a single issue through the pipeline.

```bash
simili process --issue issue.json --workflow issue-triage --dry-run
```

**Flags:**
- `--issue`: Path to issue JSON file
- `--workflow`: Workflow preset to run (default: "issue-triage")
- `--dry-run`: Run without side effects
- `--repo`, `--org`, `--number`: Override issue fields

### `simili batch`

Process multiple issues from a JSON file in batch mode. **All operations run in dry-run mode** to prevent GitHub writes.

```bash
simili batch --file issues.json --format csv --out-file results.csv --workers 5
```

**Use Cases:**
- Test bot logic on historical data without spamming repositories
- Generate reports showing similarity analysis and duplicate detection
- Analyze issues from repositories where you lack write access
- Bulk identify transfer recommendations and quality scores

**Flags:**
- `--file` (required): Path to JSON file with array of issues
- `--out-file`: Output file path (stdout if not specified)
- `--format`: Output format: `json` or `csv` (default: `json`)
- `--workers`: Number of concurrent workers (default: 1)
- `--workflow`: Workflow preset (default: "issue-triage")
- `--collection`: Override Qdrant collection name
- `--threshold`: Override similarity threshold
- `--duplicate-threshold`: Override duplicate confidence threshold
- `--top-k`: Override max similar issues to show

**Error handling:** Worker failures (including panics) cancel all in-flight work and are reported in the batch summary. Any issue that could not be processed due to a worker failure is recorded with a non-nil error in the output — it will not silently appear as a success. The process always exits cleanly; it will not hang on GitHub Actions or other CI environments.

**Input Format:**

Create a JSON file with an array of issues:

```json
[
  {
    "org": "owner",
    "repo": "repo-name",
    "number": 123,
    "title": "Issue title",
    "body": "Issue description...",
    "state": "open",
    "labels": ["bug", "high-priority"],
    "author": "username",
    "created_at": "2026-02-10T10:00:00Z"
  }
]
```

**Output Formats:**

- **JSON**: Full pipeline results with detailed analysis
- **CSV**: Flattened summary for spreadsheet analysis

**Example Workflow:**

```bash
# 1. Index repository issues
simili index --repo ballerina-platform/ballerina-library --workers 10

# 2. Prepare test issues in batch.json
# 3. Run batch analysis
simili batch --file batch.json --format csv --out-file analysis.csv --workers 5

# 4. Review results
cat analysis.csv
```

## Configuration

Minimal `.github/simili.yaml` examples:

**Zero-config (GitHub Models — no extra secrets):**
```yaml
embedding:
  provider: github_models

llm:
  provider: github_models

defaults:
  similarity_threshold: 0.65
  max_similar_to_show: 5
```

**With Qdrant and Gemini:**
```yaml
qdrant:
  url: "${QDRANT_URL}"
  api_key: "${QDRANT_API_KEY}"
  collection: "my-issues"

embedding:
  provider: "gemini"
  api_key: "${GEMINI_API_KEY}"
  model: "gemini-embedding-001"

llm:
  provider: "gemini"
  api_key: "${GEMINI_API_KEY}"
  model: "gemini-2.5-flash"

defaults:
  similarity_threshold: 0.65
  max_similar_to_show: 5
```

Notes:
- `llm.model` defaults to `gemini-2.5-flash` (Gemini) or `gpt-4o-mini` (GitHub Models) when omitted.
- `llm.api_key` can be omitted if the corresponding environment variable is set.
- You can override the model at runtime with `LLM_MODEL`.

### Search Backends

Simili supports multiple search backends for similarity detection, enabling zero-dependency setups:

- **`qdrant` (default)**: Uses Qdrant vector database for semantic search. Requires `qdrant` configuration and an `embedding` provider.
- **`github_native`**: A zero-dependency hybrid search using GitHub's native issue search API. It does not require Qdrant or embedding configuration. *Note: `simili learn` is not supported with this backend.*
- **`bm25`**: A local keyword-based search.

To configure the backend, set it in your `simili.yaml`:

```yaml
search:
  backend: "github_native" # options: "qdrant" (default), "github_native", "bm25"
```

### `simili auto-close`

Scan all open issues labelled `potential-duplicate` and close those whose grace period has expired with no human activity. Closed issues are relabelled from `potential-duplicate` → `duplicate`.

```bash
simili auto-close --repo owner/repo --grace-period-minutes 60
```

**Flags:**
- `--repo` (required): Target repository (`owner/name`); falls back to `GITHUB_REPOSITORY` env var
- `--grace-period-minutes`: Override the grace period in minutes for this run (see precedence below)
- `--dry-run`: Print what would be closed without making any changes
- `--config`: Path to `simili.yaml` (auto-discovered if omitted)

**Grace period precedence** (highest → lowest):

| Source | How to set |
|--------|-----------|
| `--grace-period-minutes` CLI flag | Pass at runtime — overrides everything |
| `auto_close.grace_period_hours` in `simili.yaml` | Persistent per-repo config |
| Built-in default | 72 hours (3 days) |

**`simili.yaml` configuration:**

```yaml
auto_close:
  grace_period_hours: 48   # default: 72
  dry_run: false
```

**Human activity signals** — any of these prevent auto-close:
1. A negative reaction (👎 or 😕) on the bot's triage comment by a non-bot user.
2. The issue was reopened by a human after the `potential-duplicate` label was applied.
3. A non-bot comment posted after the label was applied.

**GitHub Actions usage** — the `auto-close.yml` workflow runs daily at 10:00 UTC and can be triggered manually via `workflow_dispatch` with an optional `grace_period_minutes` input:

```yaml
# Trigger from GitHub UI or gh CLI:
gh workflow run auto-close.yml -f grace_period_minutes=60 -f dry_run=false
```

Leaving `grace_period_minutes` empty uses the value from `simili.yaml` (or the 72 h default).

## Development

```bash
# Clone the repository
git clone https://github.com/similigh/simili-bot.git
cd simili-bot

# Build
go build ./...

# Run tests
go test ./...

# Lint
go vet ./...
```

## License

This project is licensed under the Apache License 2.0 — see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Made by the Simili Team
</p>
