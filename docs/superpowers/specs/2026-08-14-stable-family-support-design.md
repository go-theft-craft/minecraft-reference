# Stable Minecraft family support

## Goal

`mcreference` will support one tested stable release from every Java Edition
release family from 1.0 onward. The selected release is the latest stable patch
in its family. A weekly workflow keeps those selections current.

The command will produce named sources and symbol indexes for every side that
Mojang publishes in the version metadata. Releases 1.0 and 1.1 are client-only
because their metadata has no server download. A successful process exit alone
does not qualify a release as supported. Every available side must pass output
validation before the release appears in the embedded configuration or README.

## Initial version matrix

| Family | Tested release | Minimum JDK | Mapping source |
| --- | --- | ---: | --- |
| 1.0 | `1.0` | 8 | Pinned MCP mappings |
| 1.1 | `1.1` | 8 | Pinned MCP mappings |
| 1.2 | `1.2.5` | 8 | Pinned MCP mappings |
| 1.3 | `1.3.2` | 8 | Pinned MCP mappings |
| 1.4 | `1.4.7` | 8 | Pinned MCP mappings |
| 1.5 | `1.5.2` | 8 | Pinned MCP mappings |
| 1.6 | `1.6.4` | 8 | Pinned MCP mappings |
| 1.7 | `1.7.10` | 8 | Pinned MCP mappings |
| 1.8 | `1.8.9` | 8 | Pinned MCP mappings |
| 1.9 | `1.9.4` | 8 | Pinned MCP mappings |
| 1.10 | `1.10.2` | 8 | Pinned MCP mappings |
| 1.11 | `1.11.2` | 8 | Pinned MCP mappings |
| 1.12 | `1.12.2` | 8 | Pinned MCP mappings |
| 1.13 | `1.13.2` | 8 | Pinned MCP mappings |
| 1.14 | `1.14.4` | 8 | Mojang client and server mappings |
| 1.15 | `1.15.2` | 8 | Mojang client and server mappings |
| 1.16 | `1.16.5` | 8 | Mojang client and server mappings |
| 1.17 | `1.17.1` | 16 | Mojang client and server mappings |
| 1.18 | `1.18.2` | 17 | Mojang client and server mappings |
| 1.19 | `1.19.4` | 17 | Mojang client and server mappings |
| 1.20 | `1.20.6` | 21 | Mojang client and server mappings |
| 1.21 | `1.21.11` | 21 | Mojang client and server mappings |
| 26.1 | `26.1.2` | 25 | Names distributed with the game |
| 26.2 | `26.2` | 25 | Names distributed with the game |

The Mojang version manifest determines the release list. The version metadata
determines each minimum JDK and whether Mojang publishes mapping downloads.
The embedded matrix remains explicit so a new Mojang release cannot change the
behavior of an existing `mcreference` binary.

## Configuration

Extend `config.Version` with a `Family` field. Keep `Naming` as the name of one
of three strategies:

- `mcp` uses pinned legacy mapping artifacts from `tools.json`.
- `mojang` downloads the mapping declared in the selected version metadata.
- `identity` keeps names already present in the game jar.

Each `mcp` version refers to version-specific mapping artifacts. Every external
artifact has a fixed URL and SHA-256 value in `tools.json`. Before adding an
artifact, verify its source, license, archive structure, and checksum.

`mojang` uses the `client_mappings` or `server_mappings` download that matches
the requested side. The existing downloader verifies the size and SHA-1 values
from Mojang metadata.

`identity` is valid only when validation finds named `net.minecraft` output.
The command must not use it as a fallback for an obfuscated jar.

## Mapping pipeline

Move mapping selection behind one pipeline function that accepts the version,
the side, and the original analysis jar. The function returns the named jar and
the downloaded artifacts that belong in `manifest.lock.json`.

The `mcp` path generalizes the existing 1.8.9 implementation. It loads the
mapping artifacts assigned to the selected version and remaps both the client
and the server jar.

The `mojang` path downloads the side-specific ProGuard mapping. A new parser
converts class, field, and method entries to the mapping format consumed by the
jar remapper. The conversion must preserve method descriptors and reverse the
mapping direction so the output jar uses Mojang names.

The `identity` path returns the original analysis jar without remapping it.

Server executable extraction remains separate from naming. The pipeline first
extracts a bundled server jar when necessary, then applies the selected naming
strategy.

## Output validation

After decompilation and indexing, validate each version and side before writing
the final lock file. Validation checks all of these conditions:

- The named jar contains classes below `net/minecraft`.
- The source index contains records below `net/minecraft`.
- The symbol index contains classes and members below `net.minecraft`.
- Source and symbol counts meet stored minimums for that version and side.
- A sample of expected stable classes is present when the release contains
  those classes.

The first accepted compatibility run records observed counts. Store minimums
below those counts so small decompiler changes do not cause unrelated failures.
Do not lower a minimum until the changed output has been reviewed.

Write a compact compatibility report with the tested version, side, JDK,
mapping strategy, counts, and result. Do not include game jars, mappings,
decompiled sources, or generated indexes in Git or CI artifacts.

## Commands and CI

Add a task that runs the complete compatibility matrix. It accepts version and
side filters so a developer can test one family before running the full matrix.

Normal pull-request CI keeps the current unit, race, vulnerability, secret, and
build checks. Mapping parsers and validation rules use local fixtures in that
workflow.

A manual and scheduled GitHub Actions workflow runs the network-heavy matrix.
Its matrix contains one client job and one server job for every configured
release. Each job uploads only its compatibility report and logs. The workflow
fails if any side fails download verification, mapping, decompilation,
indexing, or output validation.

Before committing a version and its README row, run its client and server
compatibility tests from the proposed working tree. After push, the matrix runs
the same tests for the commit. Do not create a release tag until the complete
compatibility workflow passes for that commit.

## Weekly version updates

Add a weekly workflow and a `workflow_dispatch` trigger. The workflow runs the
version updater from the default branch and completes these steps:

1. Download Mojang's version manifest.
2. Keep entries whose type is `release` and whose family is 1.0 or newer. The
   first two numeric components form the family name.
3. Select the newest release in each family by `releaseTime`.
4. Compare the selections with the embedded configuration.
5. Stop without a commit when the selections match.
6. Build a candidate configuration when Mojang adds a family or a newer patch.
7. Run local checks and both compatibility tests for every changed family.
8. Commit the candidate configuration and generated README table to a staging
   branch only when the candidate tests pass.
9. Dispatch the complete compatibility workflow for the staging branch and
   wait for it to pass.
10. Move `automation/minecraft-versions` to the tested commit and open or update
    one pull request.

For versions that use Mojang mappings, the updater selects `mojang` only when
the metadata has both client and server mapping downloads. For a version whose
jar contains published names, the updater may select `identity` only after both
sides pass output validation. The updater does not discover or add MCP or other
community mappings.

The updater reads the minimum JDK from Mojang metadata. It installs that JDK
for candidate tests and passes its `java` and `javap` paths explicitly. If the
JDK distribution does not provide the required major, the candidate fails and
the job summary states which JDK is missing. The updater does not change the
support table or open a version pull request in that case. It also does not
weaken validation minimums when observed counts decrease.

The workflow uses the repository `GITHUB_TOKEN`. It does not use a personal
access token or a stored API key. Give each job only the permissions it needs.
The update job needs `contents: write`, `pull-requests: write`, and
`actions: write`. Pin third-party actions by commit SHA.

The repository must allow GitHub Actions to create pull requests. If that
setting is off, fail with an instruction to enable it. Do not fall back to a
personal access token.

Restrict automatic discovery, metadata, mapping, and game downloads to Mojang
hosts that the repository allows. Configured MCP tools may use separately
allowed hosts because `tools.json` pins their URLs and SHA-256 values. Reject
other hosts and redirects to other hosts. Treat every downloaded file as
untrusted input and retain the existing size and hash checks.

The pull request includes the old and new representative versions, JDK
requirements, mapping strategies, validation counts, and links to the passing
workflow runs. The workflow never merges the pull request, creates a tag, or
publishes a release.

## Generated documentation

Generate the README support table from the embedded version configuration.
Add a check mode that fails when the table differs from the configuration.
Both the normal CI workflow and the weekly updater run this check.

The generated table contains the family, tested release, minimum JDK, mapping
source, and tested sides. Text outside the generated table remains hand-written.

## README behavior

The README lists the matrix as tested family representatives. It does not claim
that every patch in a family was independently tested. It explains that the
weekly workflow replaces a representative only after the new release passes
both sides. Users must select a JDK that meets the highest requirement among
requested releases.

The JDK selection examples keep both supported methods:

- Pass `--java` and `--javap` paths from the same JDK.
- Set `JAVA_HOME`, then prepend `$JAVA_HOME/bin` to `PATH`.

The README states that `mcreference` reads `PATH`, not `JAVA_HOME` itself.

## Failure behavior

The command stops before downloads when the selected `java` or `javap` is
missing or too old. It names the release that sets the minimum JDK.

The command rejects a version with no embedded configuration. It also rejects
a configured version whose naming strategy lacks the required mapping artifact
or Mojang metadata download.

If output validation fails, the error names the version, side, failed check,
observed value, and required value. The failed run does not write a successful
lock file.

## Scope limits

This work does not claim support for snapshots, release candidates,
pre-releases, alpha versions, beta versions, or every patch within a family. It
does not use an unofficial server jar when Mojang metadata has no server
download. It does not publish Mojang jars, mapping files, decompiled sources,
or generated indexes. The release archives continue to contain only
`mcreference` and its release metadata.
