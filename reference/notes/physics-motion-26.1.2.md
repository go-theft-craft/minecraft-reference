# Entity motion constants for Java Edition 26.1.2

Read from `reference/work/sources/26.1.2/server/`, and every value marked
*dumped* was printed by `Dump26_1` running against the prepared server jar
rather than read. Recorded by: OCharnyshevich. Date: 2026-08-17.
No source text is reproduced here, only measured values and their locations.

Every transcribed constant here was confirmed twice, as M8.1's rule requires:
once from the decompiled source and once from
`javap -p -c` over the shipped jar, which is an independent path from the same
bytecode. The bytecode reading is quoted per constant below, because it also
settles the widths — an `fmul` and a `dmul` are visible where a decimal is not.

The split between what a dumper reaches and what a person transcribes falls
differently from 1.8.9's, and in both directions. Two constants that were
literals there are attributes here, so they are now dumped. Two that were
literals there are literals still.

## player

- **Gravity: `0.08`** — *dumped*, the player's base value for the gravity
  attribute. It was a `double` literal in a method body in 1.8.9 and is an
  attribute here, so the number is unchanged and the way to get it is not.
- **Step height: `0.6`** — *dumped*, the player's base value for the step-height
  attribute.
- Horizontal drag: `0.91F` = `0.9100000262260437` — from the land branch of
  `LivingEntity.travel`, which multiplies the block's friction by it. Confirmed
  in bytecode: `ldc_w float 0.91f` followed by `fmul`, so the friction product
  is formed at single width.
- Vertical drag: `0.98F` = `0.9800000190734863` — from the same branch. Confirmed
  in bytecode: `ldc_w float 0.98f` followed by `dmul`, so the constant is widened
  and the product is formed at double width against the motion. The same three
  `dmul` instructions carry the horizontal drag, which settles that one too.
- **Movement speed: `0.10000000149011612`** — *dumped*, the player's base value
  for the movement-speed attribute. It is the widened `float` 0.1F rather than
  the round decimal, and it is the player's own value: the attribute's generic
  default is `0.7`, which is what an entity that says nothing about itself gets.

## Precision

**The step height is a `double` here and a `float` there, and it still arrives as
a `float`.** The attribute holds `0.6` exactly, and `LivingEntity.maxUpStep`
narrows the attribute's value to a `float` where it is read — visible in bytecode
as `getAttributeValue:(...)D` immediately followed by `d2f`. So the number the
step-up applies is `float64(float32(0.6))` = `0.6000000238418579`, the same value
1.8.9 applies from a `float` field — reached by a different route. A consumer
that took the attribute's `0.6` and used it directly would be wrong by two parts
in 10^16 on every step.

Gravity is a `double` at every point: the attribute holds `0.08` and the land
branch subtracts the attribute's value without narrowing.

The two drags are `float` literals multiplied into `double` motion, so Java
widens the constant to meet the motion and the product is formed at double
width. This is the shape 1.8.9 uses too, and it is the opposite of what a
"compute at single width" reading would produce.

## What is not here, and why

Item and arrow constants are not recorded. This milestone is the player on land,
and a constant transcribed without a use is a constant nobody checks.

## The formulas around them differ from 1.8.9's

Recorded because a consumer that has these four numbers is not yet able to
reproduce a tick with them:

- The ground acceleration divides `0.21600002F` by the cube of the **block's own
  friction**. 1.8.9 divides `0.16277136F` by the cube of the **friction
  product** — the block's friction already multiplied by `0.91F`. Confirmed in
  bytecode: `getSpeed()F`, `ldc_w float 0.21600002f`, then `fmul fmul fdiv fmul`,
  so the cube, the division, and the product with the speed are all at single
  width.
- The input vector is normalized at `double` width against a threshold of
  `1.0E-7`, where 1.8.9 works at `float` width against `1.0E-4F`.
- The trigonometry table is unchanged — bit for bit identical to 1.8.9's 65,536
  entries, verified by comparing the dumped tables — but the index into it is
  not. This version multiplies by the `double` `10430.378350470453` and
  truncates through a `long`; 1.8.9 multiplies by the `float` `10430.378` and
  truncates through an `int`.
- The degrees-to-radians conversion is a single pre-divided `float` constant.
  1.8.9's heading multiplies by a `float` pi and then divides by 180, which is a
  different number at some angles; its jump impulse uses the pre-divided form.

## Block slipperiness

Dumped for all 1,168 registered blocks. Five differ from the `0.6` default:
`ice` and `packed_ice` and `frosted_ice` at `0.98`, `blue_ice` at `0.989`, and
`slime_block` at `0.8`. Soul sand is `0.6` — it slows a body through a different
mechanism, as it does in 1.8.9.

The block that 1.8.9 registers as `slime` is `slime_block` here. A consumer
carrying block names across versions has to translate rather than assume.
