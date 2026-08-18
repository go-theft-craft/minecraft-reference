<p align="center">
  <img src="docs/assets/logo.png" width="350" alt="minecraft-reference logo">
</p>

# minecraft-reference

`mcreference` prepares local Java Edition reference material for independent
Minecraft implementations.

The command downloads artifacts from Mojang, verifies them, applies legacy
names where needed, decompiles them, and writes searchable symbol indexes. It
includes a tested representative from each supported stable Java Edition
family.

## Supported versions

Each row lists one tested representative for a stable release family. The
Compatibility workflow tests each representative with the exact JDK major in
the `Minimum JDK` column. This is the higher of Minecraft's declared Java
requirement and Java 17, which the pinned Vineflower decompiler requires. For
local runs, both `java` and `javap` can use that JDK major or a newer one. If
one command prepares several versions, use a JDK that meets the highest value
among those rows. `Released` comes from Mojang's release timestamp, and
`Verified` records the latest accepted compatibility run; both use UTC dates.
The newest releases appear first.

<!-- BEGIN GENERATED SUPPORTED VERSIONS -->
| Family | Tested release | Released | Verified | Minimum JDK | Mapping source | Tested sides |
| --- | --- | --- | --- | ---: | --- | --- |
| 26.2 | `26.2` | 2026-06-16 | 2026-08-14 | 25 | Names distributed with the game | client and server |
| 26.1 | `26.1.2` | 2026-04-09 | 2026-08-17 | 25 | Names distributed with the game | client and server |
| 1.21 | `1.21.11` | 2025-12-09 | 2026-08-14 | 21 | Mojang client and server mappings | client and server |
| 1.20 | `1.20.6` | 2024-04-29 | 2026-08-14 | 21 | Mojang client and server mappings | client and server |
| 1.19 | `1.19.4` | 2023-03-14 | 2026-08-14 | 17 | Mojang client and server mappings | client and server |
| 1.18 | `1.18.2` | 2022-02-28 | 2026-08-14 | 17 | Mojang client and server mappings | client and server |
| 1.17 | `1.17.1` | 2021-07-06 | 2026-08-14 | 17 | Mojang client and server mappings | client and server |
| 1.16 | `1.16.5` | 2021-01-14 | 2026-08-14 | 17 | Mojang client and server mappings | client and server |
| 1.15 | `1.15.2` | 2020-01-17 | 2026-08-14 | 17 | Mojang client and server mappings | client and server |
| 1.14 | `1.14.4` | 2019-07-19 | 2026-08-14 | 17 | Mojang client and server mappings | client and server |
| 1.13 | `1.13.2` | 2018-10-22 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.12 | `1.12.2` | 2017-09-18 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.11 | `1.11.2` | 2016-12-21 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.10 | `1.10.2` | 2016-06-23 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.9 | `1.9.4` | 2016-05-10 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.8 | `1.8.9` | 2015-12-03 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.7 | `1.7.10` | 2014-05-14 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.6 | `1.6.4` | 2013-09-19 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.5 | `1.5.2` | 2013-04-25 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.4 | `1.4.7` | 2012-12-27 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.3 | `1.3.2` | 2012-08-15 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.2 | `1.2.5` | 2012-03-29 | 2026-08-14 | 17 | Pinned MCP mappings | client and server |
| 1.1 | `1.1` | 2012-01-11 | 2026-08-14 | 17 | Pinned MCP mappings | client |
| 1.0 | `1.0` | 2011-11-17 | 2026-08-14 | 17 | Pinned MCP mappings | client |
<!-- END GENERATED SUPPORTED VERSIONS -->

Java Edition 1.0 and 1.1 are client-only because Mojang's per-version metadata
does not provide server jars for those releases.

## Use a release archive

Release archives support Linux, macOS, and Windows on amd64 and arm64. Download
the archive for your system from the
[latest release](https://github.com/go-theft-craft/minecraft-reference/releases/latest):

| System | amd64 | arm64 |
| --- | --- | --- |
| Linux | `mcreference_VERSION_linux_amd64.tar.gz` | `mcreference_VERSION_linux_arm64.tar.gz` |
| macOS | `mcreference_VERSION_darwin_amd64.tar.gz` | `mcreference_VERSION_darwin_arm64.tar.gz` |
| Windows | `mcreference_VERSION_windows_amd64.zip` | `mcreference_VERSION_windows_arm64.zip` |

On Linux or macOS, extract the archive and run the binary:

```bash
tar -xzf mcreference_VERSION_SYSTEM_ARCH.tar.gz
./mcreference version
./mcreference prepare --versions 1.8.9,26.1.2 --sides client,server \
	--java "$JAVA_HOME/bin/java" --javap "$JAVA_HOME/bin/javap"
```

On Windows, extract the ZIP archive and run these commands in PowerShell:

```powershell
.\mcreference.exe version
.\mcreference.exe prepare --versions 1.8.9,26.1.2 --sides client,server `
	--java "$env:JAVA_HOME\bin\java.exe" --javap "$env:JAVA_HOME\bin\javap.exe"
```

Replace `VERSION`, `SYSTEM`, and `ARCH` with the values from the downloaded
archive name. Each release also provides `checksums.txt` and an SBOM for every
archive. The archives contain the Go binary, so they do not require Go or
Devbox.

Before it downloads files, `prepare` checks that both `java` and `javap` exist
and meet the highest Java requirement among the requested Minecraft versions.
Both executables must come from the same JDK.

### Select a JDK

By default, `mcreference` runs `java` and `javap` from `PATH`. When multiple
JDKs are installed, pass both executable paths explicitly:

```bash
jdk=/opt/jdk-25
./mcreference prepare --versions 26.1.2 --sides server \
  --java "$jdk/bin/java" \
  --javap "$jdk/bin/javap"
```

On Windows, use the executable paths from the same JDK:

```powershell
$Jdk = "C:\Program Files\Eclipse Adoptium\jdk-25"
.\mcreference.exe prepare --versions 26.1.2 --sides server `
  --java "$Jdk\bin\java.exe" `
  --javap "$Jdk\bin\javap.exe"
```

You can instead set `JAVA_HOME` and prepend its `bin` directory to `PATH`:

```bash
export JAVA_HOME=/opt/jdk-25
export PATH="$JAVA_HOME/bin:$PATH"
./mcreference prepare --versions 26.1.2 --sides server
```

`mcreference` does not read `JAVA_HOME` itself. Updating `PATH` makes its
default `java` and `javap` commands use that JDK. Explicit flags are less
dependent on the shell environment and are preferable in scripts and CI.

[![Terminal demo of mcreference preparing Minecraft 26.1.2 server reference files](docs/assets/mcreference-terminal-demo.gif)](docs/assets/mcreference-terminal-demo.mp4)

The demo compresses a real run that produced 4,779 source records and 101,759
symbol records. Select the image to open the MP4 version.

## Build from source

```bash
go install github.com/go-theft-craft/minecraft-reference/cmd/mcreference@latest
mcreference prepare --versions 1.8.9,26.1.2 --sides client,server
```

The binary includes its supported-version and tool configuration. Use
`--config-dir` to replace both `versions.json` and `tools.json`. Game jars,
mappings, Java sources, and generated indexes stay below the selected
workspace and must not be committed or published.

A custom version entry also needs a reviewed naming strategy. Do not use
`identity` for an obfuscated Minecraft jar; the command can finish while
producing an incomplete index.

The development environment supplies OpenJDK through Devbox:

```bash
devbox run -- task reference:prepare VERSIONS=1.8.9,26.1.2 SIDES=client,server
```

See [`reference/README.md`](reference/README.md) for the workspace layout and
cleanup rules.

## Version updates

Weekly automation checks Mojang's manifest for a new stable representative in
each family. It tests every available side, creates a staging commit, and runs
the complete Compatibility workflow against that commit. Only after
Compatibility passes does the automation open or update a pull request for
human review. The updater does not publish tags or releases.
