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

The embedded catalog contains one tested representative for each supported
stable family. The table in the [project README](../README.md#supported-versions)
lists each representative, its tested sides, and its minimum JDK. Versions 1.0
and 1.1 support only the client side because Mojang's per-version metadata does
not contain server jar downloads for them.

The catalog's Java value is the minimum accepted major for both `java` and
`javap`. The Compatibility workflow tests each version with that exact major.
A local run can use newer executables. If `JAVA_HOME` selects the intended JDK,
pass both paths explicitly:

```bash
mcreference prepare --versions 1.8.9,26.1.2 --sides client,server \
	--java "$JAVA_HOME/bin/java" --javap "$JAVA_HOME/bin/javap"
```

As an alternative, add the JDK's `bin` directory to `PATH` and omit the two
flags. The `JAVA` and `JAVAP` Task variables expose the same selection for
`reference:prepare` and `compatibility`.

The workflow downloads Java classpath libraries. It skips assets, native
classifiers, logging configuration, and native launch files. The workflow does
not execute Minecraft classes.

Stable families through 1.13 use pinned MCP mappings. Families 1.14 through
1.21 use Mojang's client and server mappings. The 26.x releases use readable
names distributed with the game. Vineflower writes decompiled sources, and
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
