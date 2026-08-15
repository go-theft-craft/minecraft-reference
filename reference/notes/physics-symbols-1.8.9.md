# Physics symbol identities for Java Edition 1.8.9 server

Generated from `reference/work/index/1.8.9/server/symbols.jsonl`.
Mapping source: pinned MCP mappings (`mcp-1.8.9-srg`, `mcp_stable-22-1.8.9`).
Recorded by: OCharnyshevich. Date: 2026-08-15.

Server jar: `https://launcher.mojang.com/v1/objects/b58b2ceb36e01bcd8dbf49c8fb66c55a9f0676cd/server.jar`,
SHA-256 `c18e4245073aaff580eb7359902f0251436568b1647a9e443a924cdb73fa8312`.

## Block class

- Owner: `net.minecraft.block.Block`
- Purpose: declares both the block registry and the slipperiness field.

## Block registry field

- Name: `blockRegistry`
- Descriptor: `Lnet/minecraft/util/RegistryNamespacedDefaultedByKey;`
- Purpose: the static registry iterated to reach every registered block.

## Block slipperiness field

- Name: `slipperiness`
- Descriptor: `F`
- Purpose: the horizontal friction multiplier applied when an entity stands on the block.

## Registry name method

- Owner: `net.minecraft.util.RegistryNamespaced`
- Name: `getNameForObject`
- Descriptor: `(Ljava/lang/Object;)Ljava/lang/Object;`
- Purpose: maps a registered block back to its registry name.

## Trigonometry table

- Owner: `net.minecraft.util.MathHelper`
- Name: `SIN_TABLE`
- Descriptor: `[F`
- Purpose: the precomputed sine table every rotation reads.

## Bootstrap

- Owner: `net.minecraft.init.Bootstrap`
- Name: `register`
- Descriptor: `()V`
- Purpose: static registration that populates the block registry.

## Notes for the dumper

- `blockRegistry` is `public static final`, but its declared type is
  `RegistryNamespacedDefaultedByKey`, which inherits `getNameForObject` from
  `RegistryNamespaced`. The dumper resolves the method on the superclass.
- `SIN_TABLE` is `private static final`, so the dumper must call
  `setAccessible(true)` before reading it.
- `slipperiness` is a `public` instance field, so no accessibility change is
  needed to read it per registered block.

## Review

No decompiled source is reproduced here. These are identities only.
