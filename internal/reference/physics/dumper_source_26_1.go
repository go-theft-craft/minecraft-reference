package physics

// dump261Source is the dumper compiled and run against a prepared 26.1.2 server
// jar.
//
// It is typed rather than reflective almost throughout, which is the difference
// from the 1.8.9 dumper. That version keeps its block registry and its
// slipperiness field private and gives no public way to reach either, so
// reflection was the only way in. Here a block's friction, the registry, and the
// attribute defaults are all public, so naming the types lets javac check the
// whole program against the jar it will run on. Only the trigonometry table is
// still private — along with the default friction, which lives on the block
// properties because a block in this version cannot be constructed without an
// id — and those two are the only things this reaches by reflection.
//
// That choice paid for itself on the first compile. The registry key type is
// Identifier in this version and ResourceLocation in every earlier one, and
// javac said so — where a reflective dumper would have bootstrapped the game for
// a minute and then thrown.
//
// It lives in a Go file rather than a committed .java file for the same reason
// the others do: the repository tracks no .java under internal/, so decompiled
// output can never be committed by accident.
//
// What it cannot reach, and why. Two of the four motion constants a consumer
// needs are literals inside the movement method — the vertical drag and the
// friction constant the horizontal drag is built from — and no reflective or
// typed read finds a literal in a method body. They are transcribed into the
// dataset, twice-confirmed, exactly as 1.8.9's twelve were. The other two moved
// the other way and are no longer literals at all: gravity and step height are
// entity attributes in this version, so this dumper prints their defaults on
// standard error, where the transcription can be checked against them rather
// than merely believed.
const dump261Source = `import java.io.PrintStream;
import java.lang.reflect.Field;
import java.util.Map;
import java.util.TreeMap;

import net.minecraft.SharedConstants;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.resources.Identifier;
import net.minecraft.server.Bootstrap;
import net.minecraft.util.Mth;
import net.minecraft.world.entity.EntityType;
import net.minecraft.world.entity.ai.attributes.AttributeSupplier;
import net.minecraft.world.entity.ai.attributes.Attributes;
import net.minecraft.world.entity.ai.attributes.DefaultAttributes;
import net.minecraft.world.level.block.Block;
import net.minecraft.world.level.block.state.BlockBehaviour;

public final class Dump26_1 {
    public static void main(String[] arguments) throws Exception {
        // Held before the game starts. Bootstrapping installs a logging
        // framework that takes System.out over, and a document written through
        // that arrives wrapped in log decoration and parses as nothing.
        PrintStream out = System.out;
        PrintStream diagnostics = System.err;

        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        // Touching Blocks runs the static block that registers every block.
        // Reading the registry without it finds it empty.
        Class.forName("net.minecraft.world.level.block.Blocks");

        Map<String, Float> slipperiness = new TreeMap<String, Float>();
        for (Block block : BuiltInRegistries.BLOCK) {
            Identifier key = BuiltInRegistries.BLOCK.getKey(block);
            if (key == null) {
                throw new IllegalStateException("unregistered block: " + block);
            }
            slipperiness.put(key.getPath(), block.getFriction());
        }
        if (slipperiness.isEmpty()) {
            throw new IllegalStateException("the block registry is empty");
        }

        // The default is what untouched properties carry, which is the
        // definition of it rather than a reading of one block that happens not
        // to override it. It is read off the properties rather than off a block
        // built from them, because a block in this version refuses to be built
        // without an id: the properties are the only place the default exists on
        // its own.
        Field frictionField = BlockBehaviour.Properties.class.getDeclaredField("friction");
        frictionField.setAccessible(true);
        float defaultSlipperiness = frictionField.getFloat(BlockBehaviour.Properties.of());

        Field sinField = Mth.class.getDeclaredField("SIN");
        sinField.setAccessible(true);
        float[] sinTable = (float[]) sinField.get(null);

        // Not part of the document: the two constants a consumer still has to
        // transcribe are literals in a method body, and these are what the other
        // half of that record must agree with.
        //
        // The player's own defaults, not the attributes' generic ones. An
        // attribute carries a default for an entity that says nothing about it,
        // and the player says something about several: its movement speed is a
        // tenth where the generic default is seven tenths, so printing the
        // generic value would hand the transcription a plausible wrong number.
        AttributeSupplier player = DefaultAttributes.getSupplier(EntityType.PLAYER);
        diagnostics.println("player gravity " + player.getBaseValue(Attributes.GRAVITY));
        diagnostics.println("player stepHeight " + player.getBaseValue(Attributes.STEP_HEIGHT));
        diagnostics.println("player movementSpeed " + player.getBaseValue(Attributes.MOVEMENT_SPEED));
        diagnostics.println("attribute gravity " + Attributes.GRAVITY.value().getDefaultValue());
        diagnostics.println("attribute stepHeight " + Attributes.STEP_HEIGHT.value().getDefaultValue());

        StringBuilder document = new StringBuilder();
        document.append("{\"defaultSlipperiness\":").append(Float.toString(defaultSlipperiness));
        document.append(",\"blockSlipperiness\":{");
        boolean first = true;
        for (Map.Entry<String, Float> entry : slipperiness.entrySet()) {
            if (!first) {
                document.append(',');
            }
            first = false;
            document.append('"').append(entry.getKey()).append("\":")
                    .append(Float.toString(entry.getValue()));
        }
        document.append("},\"sinTableBase64\":\"").append(encode(sinTable)).append("\"}");
        out.print(document);
        out.flush();
    }

    private static String encode(float[] values) {
        byte[] buffer = new byte[values.length * 4];
        for (int index = 0; index < values.length; index++) {
            int bits = Float.floatToRawIntBits(values[index]);
            buffer[index * 4] = (byte) (bits);
            buffer[index * 4 + 1] = (byte) (bits >>> 8);
            buffer[index * 4 + 2] = (byte) (bits >>> 16);
            buffer[index * 4 + 3] = (byte) (bits >>> 24);
        }
        return java.util.Base64.getEncoder().encodeToString(buffer);
    }
}
`
