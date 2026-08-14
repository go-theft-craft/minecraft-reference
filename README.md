# minecraft-reference

`mcreference` prepares local Java Edition reference material for independent
Minecraft implementations.

The command downloads artifacts from Mojang, verifies them, applies legacy
names where needed, decompiles them, and writes searchable symbol indexes. It
supports Java Edition 1.8.9 and 26.1.2.

## Supported versions

<!-- BEGIN GENERATED SUPPORTED VERSIONS -->
| Family | Tested release | Minimum JDK | Mapping source | Tested sides |
| --- | --- | ---: | --- | --- |
| 1.0 | `1.0` | 8 | Pinned MCP mappings | client |
| 1.1 | `1.1` | 8 | Pinned MCP mappings | client |
| 1.2 | `1.2.5` | 8 | Pinned MCP mappings | client and server |
| 1.3 | `1.3.2` | 8 | Pinned MCP mappings | client and server |
| 1.4 | `1.4.7` | 8 | Pinned MCP mappings | client and server |
| 1.5 | `1.5.2` | 8 | Pinned MCP mappings | client and server |
| 1.6 | `1.6.4` | 8 | Pinned MCP mappings | client and server |
| 1.7 | `1.7.10` | 8 | Pinned MCP mappings | client and server |
| 1.8 | `1.8.9` | 8 | Pinned MCP mappings | client and server |
| 1.9 | `1.9.4` | 8 | Pinned MCP mappings | client and server |
| 1.10 | `1.10.2` | 8 | Pinned MCP mappings | client and server |
| 1.11 | `1.11.2` | 8 | Pinned MCP mappings | client and server |
| 1.12 | `1.12.2` | 8 | Pinned MCP mappings | client and server |
| 1.13 | `1.13.2` | 8 | Pinned MCP mappings | client and server |
| 1.14 | `1.14.4` | 8 | Mojang client and server mappings | client and server |
| 1.15 | `1.15.2` | 8 | Mojang client and server mappings | client and server |
| 1.16 | `1.16.5` | 8 | Mojang client and server mappings | client and server |
| 1.17 | `1.17.1` | 16 | Mojang client and server mappings | client and server |
| 1.18 | `1.18.2` | 17 | Mojang client and server mappings | client and server |
| 1.19 | `1.19.4` | 17 | Mojang client and server mappings | client and server |
| 1.20 | `1.20.6` | 21 | Mojang client and server mappings | client and server |
| 1.21 | `1.21.11` | 21 | Mojang client and server mappings | client and server |
| 26.1 | `26.1.2` | 25 | Names distributed with the game | client and server |
| 26.2 | `26.2` | 25 | Names distributed with the game | client and server |
<!-- END GENERATED SUPPORTED VERSIONS -->

## Run a release

Install JDK 25 first. Both `java` and `javap` must be available on `PATH`.

Download the archive for your system from the
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
./mcreference prepare --versions 1.8.9,26.1.2 --sides client,server
```

On Windows, extract the ZIP archive and run these commands in PowerShell:

```powershell
.\mcreference.exe version
.\mcreference.exe prepare --versions 1.8.9,26.1.2 --sides client,server
```

Replace `VERSION`, `SYSTEM`, and `ARCH` with the values from the downloaded
archive name. Each release also provides `checksums.txt` and an SBOM for every
archive.

Before it downloads files, `prepare` checks that both `java` and `javap` exist
and meet the Java requirement for every requested Minecraft version.

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
