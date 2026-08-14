# Stable Minecraft Family Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Support one tested stable Java Edition release from every release family, update representatives weekly, and advertise only versions whose available sides pass full compatibility tests.

**Architecture:** An explicit embedded catalog remains the source of truth for released binaries. Three naming strategies convert legacy Tiny v1 mappings, Mojang ProGuard mappings, or published game names into one named-jar pipeline. A maintenance command discovers candidates, generates documentation, and supplies CI matrices, while compatibility reports provide the acceptance gate.

**Tech Stack:** Go 1.26, Java and `javap`, SpecialSource, Vineflower, Taskfile, Devbox, GitHub Actions, Mojang version metadata, MCP mapping files

## Global constraints

- Include only Mojang manifest entries whose type is `release`.
- Select the newest stable patch in each numeric family from 1.0 onward.
- Treat the first two numeric version components as the family name.
- Test every side that Mojang metadata publishes. Versions `1.0` and `1.1` are client-only.
- Never publish Minecraft jars, mapping files, decompiled sources, or generated indexes.
- Pin every non-Mojang mapping URL and SHA-256 value.
- Reject unknown download hosts and redirects to unknown hosts.
- Use Mojang's required Java major when metadata provides it. Pass `--java` and `--javap` from the same JDK.
- Do not advertise a candidate or open its pull request until all required compatibility jobs pass.
- The weekly workflow may open a pull request. It must not merge, tag, or release.
- Use `GITHUB_TOKEN`; do not add a PAT, API key, or long-lived credential.
- Leave implementation changes uncommitted unless the user explicitly asks for commits.

---

### Task 1: Model release families, sides, mappings, and validation

**Files:**
- Modify: `internal/reference/config/config.go`
- Modify: `internal/reference/config/config_test.go`
- Modify: `internal/reference/config/defaults/versions.json`

**Interfaces:**
- Produces: `config.Version`, `config.Mapping`, `config.Validation`, `Version.SupportsSide(string) bool`, `config.ReadVersionFile`, and `config.WriteVersionFile`.
- Consumes: Existing `LoadVersions`, `RequireVersion`, and embedded JSON loading.

- [ ] **Step 1: Write failing configuration tests**

Add table-driven tests for a complete entry, duplicate families, unknown sides,
missing mapping data, and empty validation limits. Use this fixture shape:

```go
data := []byte(`{
  "versions": [{
    "id": "1.7.10",
    "family": "1.7",
    "java": 8,
    "naming": "mcp",
    "mapping": {"tool": "mcp-1.7.10-tiny", "format": "tiny-v1"},
    "sides": {
      "client": {"min_sources": 100, "min_symbols": 1000, "required_classes": ["Minecraft"]},
      "server": {"min_sources": 100, "min_symbols": 1000, "required_classes": ["MinecraftServer"]}
    }
  }]
}`)
```

Assert that `SupportsSide("client")` is true, `SupportsSide("invalid")` is
false, and malformed entries return errors that name the version and field.

- [ ] **Step 2: Run the configuration tests and confirm failure**

Run:

```bash
devbox run -- go test ./internal/reference/config
```

Expected: compilation fails because the new fields and `SupportsSide` do not
exist.

- [ ] **Step 3: Add the catalog types and validation**

Use these types:

```go
type Mapping struct {
	Format    string `json:"format"`
	Tool      string `json:"tool,omitempty"`
	SRGTool   string `json:"srg_tool,omitempty"`
	NamesTool string `json:"names_tool,omitempty"`
}

type Validation struct {
	MinSources      int      `json:"min_sources"`
	MinSymbols      int      `json:"min_symbols"`
	RequiredClasses []string `json:"required_classes"`
}

type Version struct {
	ID      string                `json:"id"`
	Family  string                `json:"family"`
	Java    int                   `json:"java"`
	Naming  string                `json:"naming"`
	Mapping *Mapping              `json:"mapping,omitempty"`
	Sides   map[string]Validation `json:"sides"`
}

func (v Version) SupportsSide(side string) bool {
	_, ok := v.Sides[side]
	return ok
}
```

Accept only `mcp`, `mojang`, and `identity`. For `mcp`, require `mapping.format`.
The `tiny-v1` format requires `mapping.tool`; `srg-csv` requires both
`mapping.srg_tool` and `mapping.names_tool`. Accept only `client` and `server`
side keys. Reject duplicate IDs and duplicate families. Require positive Java,
source, and symbol minimums.

Add `ReadVersionFile(path string) ([]Version, error)` and
`WriteVersionFile(path string, []Version) error`. Keep `LoadVersions` as the
map-returning API used by the pipeline. `WriteVersionFile` sorts by numeric
family and writes atomically.

Keep the two current versions valid with temporary minimums of `1`. Later
acceptance runs replace those values with measured floors.

- [ ] **Step 4: Run tests and inspect the configuration diff**

Run:

```bash
devbox run -- go test ./internal/reference/config
git diff --check
```

Expected: tests pass and the diff contains no whitespace errors.

### Task 2: Read release and Java metadata, then restrict download hosts

**Files:**
- Modify: `internal/reference/artifact/manifest.go`
- Modify: `internal/reference/artifact/manifest_test.go`
- Modify: `internal/reference/artifact/download.go`
- Modify: `internal/reference/artifact/download_test.go`

**Interfaces:**
- Produces: `artifact.Release`, `Resolver.ListReleases(context.Context)`, `VersionMetadata.JavaVersion`, and URL policy checks used by every download.
- Consumes: `artifact.Resolver`, `artifact.Downloader`, and Mojang's v2 manifest.

- [ ] **Step 1: Add failing release-list and Java-version tests**

Use an `httptest.Server` manifest with `release`, `snapshot`, and `old_beta`
entries. Assert that `ListReleases` preserves `ID`, `Type`, `ReleaseTime`, URL,
and SHA-1. Add version metadata with:

```json
{"id":"1.20.6","javaVersion":{"majorVersion":21},"downloads":{},"libraries":[]}
```

Assert that `DecodeVersion` returns Java major 21.

- [ ] **Step 2: Add failing host and redirect tests**

Cover these cases:

- Mojang metadata and data hosts pass.
- `libraries.minecraft.net` passes.
- configured tools on `repo.maven.apache.org`, `mcp.zeith.org`, and
  `raw.githubusercontent.com` pass only with SHA-256.
- `https://example.invalid/file` fails before a request.
- an allowed host redirecting to a denied host fails.

Use a test client whose transport rewrites allowed test URLs to local servers.
Do not weaken production host checks to make the fixtures pass.

- [ ] **Step 3: Run the artifact tests and confirm failure**

Run:

```bash
devbox run -- go test ./internal/reference/artifact
```

Expected: tests fail because release metadata, Java metadata, and URL policy do
not exist.

- [ ] **Step 4: Implement release metadata and URL policy**

Add:

```go
type Release struct {
	ID          string
	Type        string
	ReleaseTime time.Time
	URL         string
	SHA1        string
}

type JavaVersion struct {
	MajorVersion int `json:"majorVersion"`
}
```

Expose all manifest entries from `ListReleases`; filtering belongs in the
catalog package. Add `JavaVersion JavaVersion` to `VersionMetadata`.

Create one URL validator that distinguishes Mojang artifacts from configured
tools. Configure `http.Client.CheckRedirect` so every redirect target passes
the same policy. Mojang URLs require SHA-1 and configured tools require
SHA-256.

- [ ] **Step 5: Run focused and race tests**

Run:

```bash
devbox run -- go test -race ./internal/reference/artifact
```

Expected: all artifact tests pass.

### Task 3: Convert Tiny v1 and Mojang mappings to SRG

**Files:**
- Create: `internal/reference/mapping/model.go`
- Create: `internal/reference/mapping/tiny.go`
- Create: `internal/reference/mapping/tiny_test.go`
- Create: `internal/reference/mapping/proguard.go`
- Create: `internal/reference/mapping/proguard_test.go`
- Modify: `internal/reference/mapping/specialsource.go`

**Interfaces:**
- Produces: `mapping.ParseTinyV1(io.Reader)`, `mapping.ParseProGuard(io.Reader)`, and `mapping.WriteSRG(io.Writer, Mapping) error`.
- Consumes: `mapping.Remap` and SpecialSource's SRG input format.

- [ ] **Step 1: Define failing Tiny v1 tests**

Use a fixture with a class, field, constructor, method, array descriptor, and a
descriptor that refers to another mapped class:

```text
v1\tofficial\tnamed
CLASS\ta\tnet/minecraft/client/Minecraft
CLASS\tb\tnet/minecraft/world/World
FIELD\ta\tLb;\tc\tworld
METHOD\ta\t(Lb;[I)V\td\tsetWorld
```

Expect these SRG records:

```text
CL: a net/minecraft/client/Minecraft
FD: a/c net/minecraft/client/Minecraft/world
MD: a/d (Lb;[I)V net/minecraft/client/Minecraft/setWorld (Lnet/minecraft/world/World;[I)V
```

Reject duplicate classes, missing owners, unknown namespaces, malformed
descriptors, and short records.

- [ ] **Step 2: Define failing ProGuard tests**

Use this fixture:

```text
net.minecraft.client.Minecraft -> a:
    net.minecraft.world.World world -> c
    void setWorld(net.minecraft.world.World,int[]) -> d
net.minecraft.world.World -> b:
```

Expect the same SRG direction as the Tiny test. Cover line-number prefixes,
constructors, arrays, primitives, inner classes, and comments.

- [ ] **Step 3: Run mapping tests and confirm failure**

Run:

```bash
devbox run -- go test ./internal/reference/mapping
```

Expected: compilation fails because the parsers and mapping model do not exist.

- [ ] **Step 4: Implement the shared mapping model**

Use explicit records rather than format-specific strings:

```go
type Class struct {
	Source string
	Target string
}

type Field struct {
	Owner, Descriptor, Source, Target string
}

type Method struct {
	Owner, Descriptor, Source, Target string
}

type Mapping struct {
	Classes []Class
	Fields  []Field
	Methods []Method
}
```

Build the complete class map before converting member descriptors. Sort output
by record type, owner, name, and descriptor so generated SRG files are stable.

- [ ] **Step 5: Implement both parsers and SRG output**

Tiny v1 already maps `official` to `named`. Mojang ProGuard files map named
types to obfuscated types, so reverse that direction. Skip constructors because
their JVM names remain `<init>` and `<clinit>`. Convert Java source types to JVM
descriptors before remapping them.

Keep `Remap` unchanged except for accepting the generated SRG file. Preserve
its input fingerprint and atomic output behavior.

- [ ] **Step 6: Run focused tests and formatting**

Run:

```bash
devbox run -- task fmt
devbox run -- go test -race ./internal/reference/mapping
```

Expected: all mapping tests pass.

### Task 4: Apply side-specific naming strategies in the pipeline

**Files:**
- Create: `internal/reference/pipeline/naming.go`
- Create: `internal/reference/pipeline/naming_test.go`
- Modify: `internal/reference/pipeline/prepare.go`
- Modify: `internal/reference/pipeline/prepare_test.go`
- Modify: `internal/reference/artifact/manifest.go`

**Interfaces:**
- Produces: `pipeline.prepareNamedJar(context.Context, namingOptions) (namedJar, []artifact.DownloadResult, error)`.
- Consumes: `config.Version`, `artifact.VersionMetadata`, mapping parsers, `mapping.Remap`, and configured tools.

- [ ] **Step 1: Write failing strategy tests**

Test these paths with temporary jars and local mapping fixtures:

- `mcp` downloads its configured Tiny v1 tool and produces SRG before remap.
- `mojang` requires `client_mappings` for the client and `server_mappings` for
  the server.
- `identity` returns the analysis jar unchanged.
- an unknown strategy fails before decompilation.
- requesting a side absent from `Version.Sides` returns an error naming the
  version and side.

Stub the remap runner through a package variable so tests do not launch Java.

- [ ] **Step 2: Run pipeline tests and confirm failure**

Run:

```bash
devbox run -- go test ./internal/reference/pipeline
```

Expected: tests fail because `prepareNamedJar` does not exist.

- [ ] **Step 3: Implement `prepareNamedJar`**

Use a focused input type:

```go
type namingOptions struct {
	Version      config.Version
	Side         string
	AnalysisJar  string
	VersionDir   string
	ReferenceDir string
	Java         string
	Tools        map[string]config.Tool
	Metadata     artifact.VersionMetadata
	Downloader   artifact.Downloader
}
```

For `mcp` with `tiny-v1`, download `version.Mapping.Tool`, parse it, write a
deterministic SRG file, and remap. For `srg-csv`, download
`version.Mapping.SRGTool` and `version.Mapping.NamesTool`, then use the existing
composition code. Configure 1.8.9 and 1.10.2 with `srg-csv`.

For `mojang`, use the mapping download for the requested side. For `identity`,
return the original analysis jar. Record every downloaded mapping in
`manifest.lock.json`.

- [ ] **Step 4: Move naming out of `Prepare`**

Leave version resolution, library downloads, server extraction, decompilation,
and indexing in `prepare.go`. Replace its hard-coded 1.8.9 branch with one call
to `prepareNamedJar` per side.

- [ ] **Step 5: Run pipeline and full unit tests**

Run:

```bash
devbox run -- go test -race ./internal/reference/pipeline ./internal/reference/...
```

Expected: all tests pass and existing 1.8.9 behavior remains covered.

### Task 5: Validate output and write compatibility reports

**Files:**
- Create: `internal/reference/pipeline/validate.go`
- Create: `internal/reference/pipeline/validate_test.go`
- Create: `internal/reference/catalog/accept.go`
- Create: `internal/reference/catalog/accept_test.go`
- Create: `cmd/mcversionupdate/main.go`
- Create: `cmd/mcversionupdate/main_test.go`
- Modify: `internal/reference/pipeline/prepare.go`
- Modify: `internal/reference/pipeline/prepare_test.go`

**Interfaces:**
- Produces: `pipeline.CompatibilityReport`, `validateOutput`, `<version>/<side>/compatibility.json`, and `catalog.Accept`.
- Consumes: the named jar, `sources.jsonl`, `symbols.jsonl`, and `config.Validation`.

- [ ] **Step 1: Write failing validation tests**

Create small jar and JSON Lines fixtures. Cover these failures independently:

- no class path begins with `net/minecraft/`;
- no source path begins with `net/minecraft/`;
- no symbol owner begins with `net.minecraft.`;
- source count is below `MinSources`;
- symbol count is below `MinSymbols`;
- `Minecraft` or `MinecraftServer` is absent when required.

Assert errors include version, side, observed value, and required value.

- [ ] **Step 2: Define the report type**

```go
type CompatibilityReport struct {
	Version         string `json:"version"`
	Family          string `json:"family"`
	Side            string `json:"side"`
	JavaMajor       int    `json:"java_major"`
	JavapMajor      int    `json:"javap_major"`
	Naming          string `json:"naming"`
	NamedClasses    int    `json:"named_classes"`
	SourceRecords   int    `json:"source_records"`
	SymbolRecords   int    `json:"symbol_records"`
	RequiredClasses []string `json:"required_classes"`
	Passed          bool   `json:"passed"`
}
```

Do not include absolute paths, download URLs, timestamps, or machine names.

- [ ] **Step 3: Run tests and confirm failure**

Run:

```bash
devbox run -- go test ./internal/reference/pipeline
```

Expected: compilation fails because report and validation functions do not
exist.

- [ ] **Step 4: Implement validation before the lock file**

Count records with streaming scanners. Use the named jar ZIP index for class
checks. Match required class names by final class-name segment so legacy
`net.minecraft.src.Minecraft` and modern `net.minecraft.client.Minecraft` both
qualify.

Write `compatibility.json` atomically only after validation passes. Write
`manifest.lock.json` after every requested side passes. A failed version must
not retain a successful final lock file from the current run.

- [ ] **Step 5: Implement compatibility acceptance**

Implement `catalog.Accept` with this signature:

```go
func Accept(versions []config.Version, reports []pipeline.CompatibilityReport) ([]config.Version, error)
```

Require one passing report for every configured side. Set `MinSources` and
`MinSymbols` to 90 percent of the observed values, rounded down and never below
1. Preserve required class names. Add only the `accept` subcommand to
`mcversionupdate` at this stage:

```text
mcversionupdate accept --config versions.json --reports reference/work/versions --output versions.json
```

Walk the report directory without following symlinks and write the output
atomically.

- [ ] **Step 6: Run tests and inspect a controlled fixture report**

Run:

```bash
devbox run -- task fmt
devbox run -- go test -race ./internal/reference/pipeline ./internal/reference/catalog ./cmd/mcversionupdate
```

Expected: tests pass and report JSON is deterministic.

### Task 6: Add and test the initial release-family catalog

**Files:**
- Modify: `internal/reference/config/defaults/versions.json`
- Modify: `internal/reference/config/defaults/tools.json`
- Modify: `reference/provenance.md`
- Modify: `.gitignore`

**Interfaces:**
- Produces: 24 configured family representatives and their measured validation floors.
- Consumes: mapping strategies and compatibility reports from Tasks 3 through 5.

- [ ] **Step 1: Add pinned legacy mapping tools**

Use commit `7e368b355f507d62085497195cfd4c79e532c450` from
`GrylaMC/MCP_Archive`. Add these Tiny v1 files and SHA-256 values:

| Version | File below `tiny_v1s/` | SHA-256 |
| --- | --- | --- |
| `1.0` | `1.0.0/1.0-mcp50.tiny` | `ecdc480fdc85bb1e45e6073ec1e372d9cdfc00e93a729c0aae9ddbdd6d5f4f6f` |
| `1.1` | `1.1.0/1.1-mcp56.tiny` | `a76f85adabe202a03103fe1bf96a2c98c7dff3b90f6fe3825658c71a277cfb7f` |
| `1.2.5` | `1.2.5/1.2.5-mcp62.tiny` | `2cae68fdf2e3af8deb97b58812aad850dbd1ced94c58a9c439d6bc9118ba2126` |
| `1.3.2` | `1.3.2/1.3.2-mcp72.tiny` | `077bf886ddafdf92de7c8872fe550d99111d97eedeeeaadfa07a4d08775004e5` |
| `1.4.7` | `1.4.7/1.4.7-mcp726a.tiny` | `d811eb83fc20af48f87c5cb8c4b633599fc725a034741d5e1a50ad8ef81824d4` |
| `1.5.2` | `1.5.2/1.5.2-mcp751.tiny` | `54c4f0a84efd7b746128e1c2bcd511e68120f930f754287f032e4737a619d4bc` |
| `1.6.4` | `1.6.4/1.6.3-mcp811.tiny` | `3bb410c80bd56100b173c6168f33b220cff083ccbf2c2b2c440e3fc45e45420b` |
| `1.7.10` | `1.7.10/1.7.10-mcp908.tiny` | `b1297786666af95793bf1cb47392600a2e9fef4574dde4941c1081d7d94eb19f` |
| `1.9.4` | `1.9.4/1.9.4-mcp928.tiny` | `902353db37845ef16b1378b8c7a268764677782504e08ec9a5efffc85d0934f6` |
| `1.11.2` | `1.11.2/1.11.2-mcp937.tiny` | `b6bbaf5795939f260d83fbc3069938132eefdb011f95b2e924522b5b3a81fdcd` |
| `1.12.2` | `1.12.2/1.12.2-mcp942.tiny` | `b961da7dc83c644e11ba3353839bc0d5ca3ba24e0d328d17e48ba56b10690f31` |
| `1.13.2` | `1.13.2/1.13.2-mcpbotFORGE.tiny` | `2c310690a8f1b2dd829446f312022e06836bf247391ca397a6d68ea05dcd16fc` |

Keep the existing 1.8.9 tools. Add the 1.10.2 SRG export with SHA-256
`f2ce9e7cdc3e3598390a495a07cc18aa930a8b9e1480d97bfae31f29e1e75a19`
and stable 29 names with SHA-256
`0df1f289ed8417db736ee36406204309ab65e516998fa8f4139235b9127c2b7b`.

Document that MCP mappings have restrictive terms and are downloaded for local
reference generation. The project does not redistribute the mapping files or
their derived sources.

- [ ] **Step 2: Add all 24 candidate versions**

Add these representatives in ascending family order:

```text
1.0 1.1 1.2.5 1.3.2 1.4.7 1.5.2 1.6.4 1.7.10
1.8.9 1.9.4 1.10.2 1.11.2 1.12.2 1.13.2 1.14.4 1.15.2
1.16.5 1.17.1 1.18.2 1.19.4 1.20.6 1.21.11 26.1.2 26.2
```

Use Java 8 through 1.16.5, Java 16 for 1.17.1, Java 17 for 1.18.2 and
1.19.4, Java 21 for 1.20.6 and 1.21.11, and Java 25 for 26.1.2 and 26.2.
Configure only the client side for 1.0 and 1.1. Set candidate minimums to `1`
until the acceptance command replaces them.

- [ ] **Step 3: Run compatibility tests in bounded batches**

Use one shared cache and one reference directory per batch. Pass JDK 25 paths
explicitly for the initial run:

```bash
devbox run -- sh -c 'go run ./cmd/mcreference prepare --versions 1.0,1.1 --sides client --java "$(command -v java)" --javap "$(command -v javap)"'
devbox run -- sh -c 'go run ./cmd/mcreference prepare --versions 1.2.5,1.3.2,1.4.7,1.5.2 --sides client,server --java "$(command -v java)" --javap "$(command -v javap)"'
devbox run -- sh -c 'go run ./cmd/mcreference prepare --versions 1.6.4,1.7.10,1.8.9,1.9.4,1.10.2,1.11.2 --sides client,server --java "$(command -v java)" --javap "$(command -v javap)"'
devbox run -- sh -c 'go run ./cmd/mcreference prepare --versions 1.12.2,1.13.2,1.14.4,1.15.2,1.16.5,1.17.1 --sides client,server --java "$(command -v java)" --javap "$(command -v javap)"'
devbox run -- sh -c 'go run ./cmd/mcreference prepare --versions 1.18.2,1.19.4,1.20.6,1.21.11,26.1.2,26.2 --sides client,server --java "$(command -v java)" --javap "$(command -v javap)"'
```

Expected: every configured side writes a passing compatibility report. If a
mapping does not match its game jar, keep that version out of the support table
and record the exact failed check in the implementation notes. Do not replace
it with identity naming.

- [ ] **Step 4: Accept measured floors**

Run the acceptance command from Task 5:

```bash
devbox run -- go run ./cmd/mcversionupdate accept \
  --config internal/reference/config/defaults/versions.json \
  --reports reference/work/versions \
  --output internal/reference/config/defaults/versions.json
```

It sets each minimum to 90 percent of the observed count, rounded down and
never below 1. It preserves required classes `Minecraft` for clients and
`MinecraftServer` for servers.

Run the full matrix again. Expected: all reports pass against the stored
minimums.

- [ ] **Step 5: Keep generated work out of Git**

Confirm `.gitignore` excludes compatibility workspaces, jars, mappings,
sources, indexes, and local reports. Run:

```bash
git status --short
```

Expected: no downloaded Minecraft or mapping artifact appears.

### Task 7: Build the maintenance command and generated README table

**Files:**
- Modify: `internal/reference/catalog/accept.go`
- Modify: `internal/reference/catalog/accept_test.go`
- Create: `internal/reference/catalog/catalog.go`
- Create: `internal/reference/catalog/catalog_test.go`
- Create: `internal/reference/catalog/readme.go`
- Create: `internal/reference/catalog/readme_test.go`
- Modify: `cmd/mcversionupdate/main.go`
- Modify: `cmd/mcversionupdate/main_test.go`
- Modify: `README.md`
- Modify: `internal/buildcheck/reference_test.go`

**Interfaces:**
- Produces: `mcversionupdate discover`, `matrix`, `accept`, and `readme` subcommands.
- Consumes: Mojang releases, version metadata, embedded configuration, and compatibility reports.

- [ ] **Step 1: Write failing family-selection tests**

Test release ordering with `1.20.5`, `1.20.6`, `1.21`, `1.21.11`, `26.1`,
`26.1.2`, and `26.2`. Assert that the selector uses `releaseTime`, not lexical
version order. Exclude snapshots, pre-releases, old alpha, and old beta.

Use this result type:

```go
type Candidate struct {
	Family string
	Old    *config.Version
	New    config.Version
}
```

For a new release, choose `mojang` only when both requested side mappings are
present. Choose `identity` only as a candidate that still requires output
validation. Never generate `mcp` automatically.

- [ ] **Step 2: Write failing README generation tests**

Place these markers around the current support table:

```markdown
<!-- BEGIN GENERATED SUPPORTED VERSIONS -->
<!-- END GENERATED SUPPORTED VERSIONS -->
```

Assert stable family ordering, exact JDK values, readable mapping names, and
`client` rather than `client and server` for 1.0 and 1.1. Assert that check mode
returns an error when generated content differs.

- [ ] **Step 3: Write failing CLI tests**

Define these commands:

```text
mcversionupdate discover --output candidate.json
mcversionupdate matrix --config candidate.json
mcversionupdate accept --config candidate.json --reports reference/compatibility --output versions.json
mcversionupdate readme --config versions.json --file README.md --write
mcversionupdate readme --config versions.json --file README.md --check
```

Inject an HTTP client and filesystem root into `run` tests. Assert that no-change
discovery exits successfully with an empty candidate list. A missing required
JDK remains candidate metadata; CI decides whether its distribution is
available.

- [ ] **Step 4: Implement catalog selection and CLI commands**

`discover` fetches metadata only for new or replaced representatives. `matrix`
prints GitHub-compatible JSON entries with `version`, `family`, `side`, and
`java`. `accept` requires one passing report per configured side and calculates
the 90-percent floors. `readme` changes only text between the markers.

- [ ] **Step 5: Run tests and regenerate README**

Run:

```bash
devbox run -- task fmt
devbox run -- go test -race ./internal/reference/catalog ./cmd/mcversionupdate ./internal/buildcheck
devbox run -- go run ./cmd/mcversionupdate readme --config internal/reference/config/defaults/versions.json --file README.md --write
devbox run -- go run ./cmd/mcversionupdate readme --config internal/reference/config/defaults/versions.json --file README.md --check
```

Expected: tests pass and the README lists only accepted versions.

### Task 8: Add local compatibility tasks and reusable CI

**Files:**
- Modify: `Taskfile.yml`
- Create: `.github/workflows/compatibility.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`

**Interfaces:**
- Produces: `task compatibility`, `task versions:check`, and a reusable Compatibility workflow.
- Consumes: `mcreference`, `mcversionupdate matrix`, Devbox, and a JDK selected by Java major.

- [ ] **Step 1: Add local Taskfile commands**

Add `JAVA` and `JAVAP` variables to `reference:prepare`. Add:

```yaml
versions:check:
  cmds:
    - go run ./cmd/mcversionupdate readme --config internal/reference/config/defaults/versions.json --file README.md --check

compatibility:
  requires:
    vars: [VERSIONS]
  cmds:
    - task: reference:prepare
      vars:
        VERSIONS: '{{.VERSIONS}}'
        SIDES: '{{.SIDES | default "client,server"}}'
        JAVA: '{{.JAVA | default "java"}}'
        JAVAP: '{{.JAVAP | default "javap"}}'
        CONFIG_DIR: '{{.CONFIG_DIR | default ""}}'
```

Pass `CONFIG_DIR` to `mcreference prepare --config-dir`. An empty value keeps
the embedded configuration. Candidate workflows create a temporary config
directory containing candidate `versions.json` and the tracked `tools.json`.

Make `verify` run `versions:check`.

- [ ] **Step 2: Test Taskfile argument propagation**

Run one client-only version with explicit executables:

```bash
devbox run -- task compatibility VERSIONS=1.0 SIDES=client JAVA="$(command -v java)" JAVAP="$(command -v javap)"
```

Expected: preflight reports the selected Java and one passing report exists.

- [ ] **Step 3: Add the reusable Compatibility workflow**

Support `workflow_call`, `workflow_dispatch`, and a weekly schedule. Use a setup
job to emit the JSON matrix. Each matrix job installs the declared Java major
with
`actions/setup-java@b6effb05e454b25005698d916606bdc6ffcbf961` at v5.7.0 and
the `temurin` distribution. It then installs Devbox and runs one side:

```yaml
- name: Prepare reference
  env:
    JAVA_BIN: ${{ env.JAVA_HOME }}/bin/java
    JAVAP_BIN: ${{ env.JAVA_HOME }}/bin/javap
  run: >-
    devbox run -- task compatibility
    VERSIONS=${{ matrix.version }}
    SIDES=${{ matrix.side }}
    JAVA="$JAVA_BIN"
    JAVAP="$JAVAP_BIN"
```

Upload only `compatibility.json`. Do not upload the reference workspace. Use a
cache key that includes version, side, mapping-tool hashes, and Vineflower
version. Set per-job timeouts and `fail-fast: false`.

- [ ] **Step 4: Gate releases and normal CI**

Keep normal CI network-light and add the generated README check through
`task verify`. Call the reusable Compatibility workflow from `release.yml` and
make the publish job depend on both `verify` and `compatibility`.

- [ ] **Step 5: Validate workflow syntax and local checks**

Run:

```bash
devbox run -- task verify
devbox run -- task release:snapshot
```

Expected: secret scan, vulnerability scan, race tests, build, GoReleaser check,
README check, and snapshot build pass.

### Task 9: Add the weekly update pull request workflow

**Files:**
- Create: `.github/workflows/update-minecraft-versions.yml`
- Modify: `reference/provenance.md`

**Interfaces:**
- Produces: a tested `automation/minecraft-versions` pull request.
- Consumes: `mcversionupdate`, the Compatibility workflow, `GITHUB_TOKEN`, and GitHub CLI.

- [ ] **Step 1: Add discovery with read-only permissions**

Schedule the workflow once per week and allow manual dispatch. The discovery
job uses `contents: read`, checks out `main`, installs Devbox, runs `task verify`,
and writes `candidate.json`. If the candidate list is empty, write "No stable
family updates" to the job summary and stop.

- [ ] **Step 2: Test changed candidates before any push**

Build a dynamic matrix from `mcversionupdate matrix --config candidate.json`.
Create a temporary configuration directory with candidate `versions.json` and
the tracked `tools.json`. Install each candidate's Java major through the pinned
`actions/setup-java` action and the `temurin` distribution. If setup-java cannot
install the required stable JDK, fail with this summary:

```text
Minecraft VERSION requires Java MAJOR, but the configured JDK distribution does not provide it. Add that JDK before advertising this release.
```

Run both available sides with explicit `$JAVA_HOME/bin/java` and
`$JAVA_HOME/bin/javap`. Download all candidate reports into the acceptance job.

- [ ] **Step 3: Create and validate a staging commit**

The acceptance job runs `mcversionupdate accept`, regenerates README, runs
`task verify`, and commits only generated catalog files. Push to
`automation/minecraft-versions-staging` with `contents: write`. Dispatch the
Compatibility workflow on that branch with `actions: write`, then poll its run
with `gh run watch --exit-status`.

Do not expose the token in command arguments or logs. Set it only through:

```yaml
env:
  GH_TOKEN: ${{ github.token }}
```

- [ ] **Step 4: Promote the tested commit and open a pull request**

After compatibility passes, move `automation/minecraft-versions` to the tested
commit. Use `gh pr create` or `gh pr edit` with `pull-requests: write`. The body
lists old and new versions, Java majors, naming strategies, measured counts,
and workflow URLs.

If repository settings prohibit Actions from creating pull requests, fail with
an instruction to enable "Allow GitHub Actions to create and approve pull
requests." Do not request a PAT.

- [ ] **Step 5: Add workflow safety tests and documentation**

Extend `internal/buildcheck/reference_test.go` to assert SHA-pinned actions,
least-privilege top-level permissions, no `pull_request_target`, no secret
fallback, and no release command in the updater. Update provenance with the
manifest URL, mapping sources, pinned commits, and update policy.

- [ ] **Step 6: Run the updater in dry-run mode**

Run:

```bash
devbox run -- go run ./cmd/mcversionupdate discover --output /tmp/mcreference-candidate.json
devbox run -- go run ./cmd/mcversionupdate matrix --config /tmp/mcreference-candidate.json
devbox run -- task verify
git diff --check
```

Expected: discovery reflects the current Mojang manifest, no credentials appear
in output, and all local checks pass.

### Task 10: Final acceptance and release readiness

**Files:**
- Modify: `README.md`
- Modify: `reference/README.md`
- Modify: `reference/provenance.md`

**Interfaces:**
- Produces: reviewed user documentation and release evidence.
- Consumes: the accepted catalog, compatibility workflow, and existing release process.

- [ ] **Step 1: Finish user-facing documentation**

Explain that the table lists one tested representative per stable family. State
that 1.0 and 1.1 are client-only because Mojang metadata lacks server jars.
Keep the explicit `--java` and `--javap` examples and the `JAVA_HOME` plus
`PATH` alternative. Explain that weekly automation opens a reviewed pull
request only after compatibility passes.

- [ ] **Step 2: Run every local quality gate**

Run:

```bash
devbox run -- task fmt
devbox run -- task verify
devbox run -- task release:snapshot
git diff --check
```

Expected: formatting, lint, secret scan, vulnerability scan, race tests, build,
README generation, GoReleaser validation, and release snapshot all pass.

- [ ] **Step 3: Run the complete compatibility workflow**

Push only when the user authorizes it. Dispatch Compatibility for the exact
commit. Expected: every configured client and server job passes, with only 1.0
and 1.1 lacking server jobs.

- [ ] **Step 4: Review the final scope**

Inspect:

```bash
git status --short
git diff --stat
git diff -- . ':!docs/assets/*'
```

Confirm that no jars, mappings, decompiled sources, indexes, credentials, or
unrelated user changes are present. Do not commit, push, tag, or release without
the user's explicit instruction.
