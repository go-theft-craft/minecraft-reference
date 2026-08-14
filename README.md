# minecraft-reference

`mcreference` prepares local Java Edition reference material for independent
Minecraft implementations.

The command downloads artifacts from Mojang, verifies them, applies legacy
names where needed, decompiles them, and writes searchable symbol indexes. It
supports Java Edition 1.8.9 and 26.1.2.

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

## Build from source

```bash
go install github.com/go-theft-craft/minecraft-reference/cmd/mcreference@latest
mcreference prepare --versions 1.8.9,26.1.2 --sides client,server
```

The binary includes its supported-version and tool configuration. Use
`--config-dir` to replace both `versions.json` and `tools.json`. Game jars,
mappings, Java sources, and generated indexes stay below the selected
workspace and must not be committed or published.

The development environment supplies OpenJDK through Devbox:

```bash
devbox run -- task reference:prepare VERSIONS=1.8.9,26.1.2 SIDES=client,server
```

See [`reference/README.md`](reference/README.md) for the workspace layout and
cleanup rules.
