# Reference workflow provenance

The workflow resolves Minecraft version metadata from Mojang's Java Edition
version manifest:

<https://piston-meta.mojang.com/mc/game/version_manifest_v2.json>

It verifies each version JSON, client jar, server jar, and Java library against
the size and SHA-1 published in that metadata. Each local lock manifest also
records the computed SHA-256.

Pinned workflow inputs:

| Input | Version | SHA-256 |
|---|---:|---|
| Vineflower | 1.12.0 | `1dfcfe974395734fa467ce620661c7623d05ba83670de0529b1fbd63ff548b9d` |
| SpecialSource | 1.11.4 | `e2cab24b1c12400ad73b15972bb21e4273a0dc7081c8b3c136ddfdd824c78518` |
| MCP 1.8.9 SRG export | 1.8.9 | `a9d6afe0e3bdb4da77a62d7cc79750c7cf53b3f0bc6cc5157f191008d0134558` |
| MCP stable export | 22 for 1.8.9 | `aeed0aaba9d159b7ce60a21e2dcc36adb249fade65ce2f76c730dd0ec7270763` |

The binary embeds the tracked values from
`internal/reference/config/defaults/tools.json`. Generated lock manifests
below `reference/work/versions/<version>/manifest.lock.json`
record the exact artifacts used in a local run. Those files remain ignored
because they contain local paths and describe restricted local artifacts.
