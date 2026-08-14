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
| MCP Archive Tiny v1 | MCP 50 for 1.0 | `ecdc480fdc85bb1e45e6073ec1e372d9cdfc00e93a729c0aae9ddbdd6d5f4f6f` |
| MCP Archive Tiny v1 | MCP 56 for 1.1 | `a76f85adabe202a03103fe1bf96a2c98c7dff3b90f6fe3825658c71a277cfb7f` |
| MCP Archive Tiny v1 | MCP 62 for 1.2.5 | `2cae68fdf2e3af8deb97b58812aad850dbd1ced94c58a9c439d6bc9118ba2126` |
| MCP Archive Tiny v1 | MCP 72 for 1.3.2 | `077bf886ddafdf92de7c8872fe550d99111d97eedeeeaadfa07a4d08775004e5` |
| MCP Archive Tiny v1 | MCP 726a for 1.4.7 | `d811eb83fc20af48f87c5cb8c4b633599fc725a034741d5e1a50ad8ef81824d4` |
| MCP Archive Tiny v1 | MCP 751 for 1.5.2 | `54c4f0a84efd7b746128e1c2bcd511e68120f930f754287f032e4737a619d4bc` |
| MCP Archive Tiny v1 | MCP 811 for 1.6.4 | `3bb410c80bd56100b173c6168f33b220cff083ccbf2c2b2c440e3fc45e45420b` |
| MCP Archive Tiny v1 | MCP 908 for 1.7.10 | `b1297786666af95793bf1cb47392600a2e9fef4574dde4941c1081d7d94eb19f` |
| MCP Archive Tiny v1 | MCP 928 for 1.9.4 | `902353db37845ef16b1378b8c7a268764677782504e08ec9a5efffc85d0934f6` |
| MCP 1.10.2 SRG export | 1.10.2 | `f2ce9e7cdc3e3598390a495a07cc18aa930a8b9e1480d97bfae31f29e1e75a19` |
| MCP stable export | 29 for 1.10.2 | `0df1f289ed8417db736ee36406204309ab65e516998fa8f4139235b9127c2b7b` |
| MCP Archive Tiny v1 | MCP 937 for 1.11.2 | `b6bbaf5795939f260d83fbc3069938132eefdb011f95b2e924522b5b3a81fdcd` |
| MCP Archive Tiny v1 | MCP 942 for 1.12.2 | `b961da7dc83c644e11ba3353839bc0d5ca3ba24e0d328d17e48ba56b10690f31` |
| MCP Archive Tiny v1 | MCPBot/Forge for 1.13.2 | `2c310690a8f1b2dd829446f312022e06836bf247391ca397a6d68ea05dcd16fc` |

The Tiny v1 mappings come from `GrylaMC/MCP_Archive` commit
`7e368b355f507d62085497195cfd4c79e532c450`. That archive warns that MCP
mappings have extremely restrictive terms. The MCP files are downloaded only
for local reference generation. This project does not redistribute mapping
files, remapped jars, or sources derived from those mappings.

The binary embeds the tracked values from
`internal/reference/config/defaults/tools.json`. Generated lock manifests
below `reference/work/versions/<version>/manifest.lock.json`
record the exact artifacts used in a local run. Those files remain ignored
because they contain local paths and describe restricted local artifacts.
