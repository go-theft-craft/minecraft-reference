# minecraft-reference

`mcreference` prepares local Java Edition reference material for independent
Minecraft implementations.

The command downloads artifacts from Mojang, verifies them, applies legacy
names where needed, decompiles them, and writes searchable symbol indexes. It
supports Java Edition 1.8.9 and 26.1.2.

```bash
go install github.com/go-theft-craft/minecraft-reference/cmd/mcreference@latest
mcreference prepare --versions 1.8.9,26.1.2 --sides client,server
```

The binary includes its supported-version and tool configuration. Use
`--config-dir` to replace both `versions.json` and `tools.json`. Game jars,
mappings, Java sources, and generated indexes stay below the selected
workspace and must not be committed or published.

The command needs `java` and `javap`. The development environment supplies
OpenJDK through Devbox:

```bash
devbox run -- task reference:prepare VERSIONS=1.8.9,26.1.2 SIDES=client,server
```

See [`reference/README.md`](reference/README.md) for the workspace layout and
cleanup rules.
