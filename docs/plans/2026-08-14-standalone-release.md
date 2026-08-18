# Standalone mcreference release implementation plan

> **Status: complete, 2026-08-18.** This repository is standalone, published,
> and released at `v1.0.1`, with `ci.yml`, `compatibility.yml`, and
> `release.yml`. Consumers run the released command by version rather than a
> copy: `minecraft-simulation`'s `Taskfile.yml` invokes
> `mcreference@{{.MCREFERENCE_VERSION}}`. The checkboxes below were never ticked
> and are not evidence; do not re-run this plan.

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development or execute this plan inline one task at a time. Steps use checkbox syntax for tracking.

**Goal:** Move `mcreference` into `minecraft-reference` and publish standalone binaries with GoReleaser.

**Architecture:** The command embeds its default Minecraft version and tool configuration. A caller selects a workspace for generated material, while the simulation module keeps only reviewed research and ignored local output.

**Tech stack:** Go 1.26.6, Devbox, Task, GoReleaser, GitHub Actions, OpenJDK 25, Vineflower, SpecialSource, and `javap`.

## Global constraints

- Keep Mojang jars, mappings, decompiled Java, indexes, and caches out of Git and release archives.
- Release only the Go command archives, checksums, and SBOMs.
- Require an explicit workspace and keep cleanup within that workspace.
- Keep `minecraft-simulation` free of Java and decompiler implementation dependencies.

---

### Task 1: Make the command standalone

**Files:**

- Modify: `cmd/mcreference/main.go`
- Modify: `internal/reference/config/config.go`
- Modify: `internal/reference/pipeline/prepare.go`
- Move: `reference/config/*.json` to `internal/reference/config/defaults/`
- Test: `cmd/mcreference/main_test.go`
- Test: `internal/reference/config/config_test.go`

**Interfaces:**

- Consumes: `--workspace`, `--reference-dir`, and optional `--config-dir` flags.
- Produces: one repository-independent `mcreference` binary with embedded defaults.

- [x] **Step 1:** Add tests for embedded configuration and version output.
- [x] **Step 2:** Embed the default JSON files and retain a directory override.
- [x] **Step 3:** Replace repository-root discovery with workspace validation.
- [x] **Step 4:** Run focused command, config, path, and cleanup tests.

### Task 2: Add release automation

**Files:**

- Create: `.goreleaser.yaml`
- Create: `.github/workflows/release.yml`
- Modify: `Taskfile.yml`
- Modify: `devbox.json`
- Modify: `.gitignore`

**Interfaces:**

- Consumes: a Git tag named `v*` and `GITHUB_TOKEN`.
- Produces: Linux, macOS, and Windows archives for amd64 and arm64, checksums, and SBOMs.

- [x] **Step 1:** Define reproducible builds with version metadata.
- [x] **Step 2:** Add Task targets for config validation, snapshots, and tagged releases.
- [x] **Step 3:** Add a tag-triggered GitHub Actions workflow with concurrency control.
- [x] **Step 4:** Run `devbox run -- task verify` and a GoReleaser snapshot.

### Task 3: Reduce the simulation repository to its own responsibility

**Files:**

- Remove: `cmd/mcreference/`
- Remove: `internal/reference/`
- Modify: `Taskfile.yml`
- Modify: `devbox.json`
- Modify: `README.md`
- Modify: `reference/README.md`

**Interfaces:**

- Consumes: the versioned `mcreference` command from `minecraft-reference`.
- Produces: a protocol-independent simulation module without Java tooling code.

- [ ] **Step 1:** Remove the copied command and its implementation packages.
- [ ] **Step 2:** keep the restricted-artifact check and ignored workspace.
- [ ] **Step 3:** Add Task wrappers that run the external command.
- [ ] **Step 4:** Run `devbox run -- task verify`.

### Task 4: Publish both repositories

**Files:** Git indexes and GitHub repository settings only.

**Interfaces:**

- Produces: public `go-theft-craft/minecraft-reference` and `go-theft-craft/minecraft-simulation` repositories on `main`.

- [ ] **Step 1:** Review both staged diffs and scan for restricted artifacts.
- [ ] **Step 2:** Commit each repository with a focused message.
- [ ] **Step 3:** Create missing GitHub repositories and push `main`.
- [ ] **Step 4:** Verify remote branch heads and GitHub Actions runs.
