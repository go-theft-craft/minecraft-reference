# minecraft-reference

`mcreference` prepares local Java Edition reference material for independent
Minecraft implementations.

The command downloads artifacts from Mojang, verifies them, applies legacy
names where needed, decompiles them, and writes searchable symbol indexes. It
supports the Java Edition versions listed below.

## Supported Minecraft versions

| Minecraft | Minimum JDK | Naming strategy | Sides |
| --- | ---: | --- | --- |
| `1.8.9` | 8 | MCP stable 22 | client and server |
| `26.1.2` | 25 | Names distributed with the game | client and server |

These are the only versions supported by the released binary. A custom
configuration does not make another version supported unless it also provides
a reviewed naming strategy and passes reference-output checks.

## Run a release

Install a JDK that meets the table above. JDK 25 can process both supported
versions.

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
and meet the highest Java requirement among the requested Minecraft versions.

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
