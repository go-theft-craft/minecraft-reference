# Local Minecraft reference workspace

`reference:prepare` accepts these Task variables:

- `VERSIONS`: comma-separated supported versions, required
- `SIDES`: `client`, `server`, or both; defaults to both
- `WORKSPACE`: workspace root; defaults to the current directory
- `REFERENCE_DIR`: workspace-local output directory; defaults to
  `reference/work`

The workflow resolves official downloads through Mojang's version manifest.
It verifies the metadata digest, artifact size, and artifact SHA-1 before use.
Tool and legacy mapping downloads use pinned SHA-256 values from the embedded
configuration. Pass `--config-dir` to load replacement files.

The workflow downloads Java classpath libraries. It skips assets, native
classifiers, logging configuration, and native launch files. The workflow does
not execute Minecraft classes.

Java 1.8.9 uses MCP stable 22 names applied through SpecialSource. Java 26.1.2
uses its distributed readable names. Vineflower writes decompiled sources and
`javap` supplies authoritative JVM descriptors.
Both indexes include only `net.minecraft.*` classes. Bundled third-party
dependencies remain available on the analysis classpath but do not become
reference sources.

The default workspace has this layout:

```text
reference/work/
  cache/                         Verified metadata and pinned tools
  versions/<version>/<side>/     Original, executable, and named jars
  sources/<version>/<side>/      Decompiled Java sources
  index/<version>/<side>/        JVM and source JSON Lines indexes
```

The command checks cached downloads before reuse. It also fingerprints the
input jar, the tool, and the mapping before it reuses a named jar or a source
tree.

All generated content stays below `REFERENCE_DIR`. This repository ignores the
default directory and restricted file extensions. Add the same rules to any
other workspace. Do not commit or publish generated content. Remove it with:

```bash
devbox run -- task reference:clean REFERENCE_DIR=reference/work
```

The cleanup command rejects paths outside the workspace, the workspace root,
and parent directories.
