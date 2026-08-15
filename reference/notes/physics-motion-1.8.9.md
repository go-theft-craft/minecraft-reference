# Entity motion constants for Java Edition 1.8.9

Read from `reference/work/sources/1.8.9/server/`. Recorded by: OCharnyshevich. Date: 2026-08-15.
These are numeric literals inside method bodies. Reflection cannot extract them.
No source text is reproduced here, only measured values and their locations.

Each entry gives the literal as declared and, where the literal is a `float`
that Java widens to `double` at the point of use, the exact `double` the game
actually computes with. A simulation that substitutes the decimal-looking value
for the widened one will drift. See "Precision" below.

## player

- Gravity: `0.08` — from `EntityLivingBase.moveEntityWithHeading`
- Horizontal drag: `0.91F` = `0.9100000262260437` — from `EntityLivingBase.moveEntityWithHeading`
- Vertical drag: `0.98F` = `0.9800000190734863` — from `EntityLivingBase.moveEntityWithHeading`
- Step height: `0.6F` = `0.6000000238418579` — from `EntityLivingBase` constructor

## item

- Gravity: `0.04F` = `0.03999999910593033` — from `EntityItem.onUpdate`
- Horizontal drag: `0.98F` = `0.9800000190734863` — from `EntityItem.onUpdate`
- Vertical drag: `0.98F` = `0.9800000190734863` — from `EntityItem.onUpdate`
- Step height: `0.0` — `EntityItem` never assigns `Entity.stepHeight`, which has no initializer

## arrow

- Gravity: `0.05F` = `0.05000000074505806` — from `EntityArrow.onUpdate`
- Horizontal drag: `0.99F` = `0.9900000095367432` — from `EntityArrow.onUpdate`
- Vertical drag: `0.99F` = `0.9900000095367432` — from `EntityArrow.onUpdate`
- Step height: `0.0` — `EntityArrow` never assigns `Entity.stepHeight`, which has no initializer

## Precision

Player gravity is the only one of these declared as a `double`, so `0.08` is
exact. Every other value is a `float` literal widened to `double` where it is
applied, which is why the widened forms above are not the round decimals.

Two of the horizontal drags are not applied directly. On the ground the game
computes the product in `float` before widening:

- player: `block slipperiness * 0.91F`
- item: `block slipperiness * 0.98F`

In air the bare constant applies. A kernel that computes that product in
`float64` will not match the game bit for bit; the multiplication has to happen
in `float32` and widen afterwards.

## Conditional values not recorded above

These belong to branches outside the common motion path and are listed so a
later reader does not mistake them for the values above:

- arrow in water: drag `0.6F` replaces `0.99F`
- arrow on hitting a block: motion scaled by `-0.1F` per axis
- item on bouncing: vertical motion scaled by `-0.5`
- player in water and in lava: separate drag and buoyancy branches entirely

## Review

Each value was read directly and independently confirmed by a second reading:
first from the decompiled sources above, then from `javap -c` disassembly of
`reference/work/versions/1.8.9/server/named.jar`, which is what produced the
exact widened `double` forms. The two readings agree on all twelve values.
Ranges are checked by `TestPhysicsDocumentRanges` in minecraft-protocol.
