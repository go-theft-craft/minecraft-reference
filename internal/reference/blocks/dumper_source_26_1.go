package blocks

// dumpBlocks261Source is the dumper compiled and run against a prepared 26.1.2
// server jar.
//
// It is typed rather than reflective, which is the difference from the 1.8.9
// dumper. That version keeps the block registry in a private field and gives no
// public way to reach it, so reflection was the only way in. Here every fact
// this needs is public — the block registry, the state registry, the state
// definition, and blocksMotion — so naming the types lets javac check the whole
// program against the jar it will run on. A version that renamed one of these
// then fails to compile, which is a far better failure than a NoSuchMethodError
// halfway through a dump.
//
// It lives in a Go file rather than a committed .java file for the same reason
// the others do: the repository tracks no .java at all, so decompiled output can
// never be committed by accident.
//
// The fact it measures is blocksMotion, which is this version's continuation of
// the material's blocksMovement that the 1.8.9 dumper reads. It resolves to
// legacySolid, computed once per state in initCache from that state's own
// collision shape, with cobweb and bamboo saplings excluded by name in the
// game's own code.
//
// It measures two more facts, both per block. Falling is FallingBlock, the
// class sand, gravel, concrete powder, the anvil, and the dragon egg extend.
//
// Climbable is the awkward one, and it is read from the jar rather than from
// the running game on purpose. This version answers it with BlockTags.CLIMBABLE,
// and a tag is not something Bootstrap binds — tags arrive from a data pack on
// a server reload, so a dumper that asked state.is(CLIMBABLE) after bootStrap
// would get false for every block in the game and report a world with no
// ladders in it. Standing a full reload up to bind one tag is a great deal of
// machinery to answer a nine-entry question. So the dumper reads the tag's own
// document out of the same jar the digest names, which is where the game reads
// it from too, and refuses a nested tag reference rather than resolving one it
// has never seen.
const dumpBlocks261Source = `import com.google.gson.JsonElement;
import com.google.gson.JsonObject;
import com.google.gson.JsonParser;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import net.minecraft.SharedConstants;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.server.Bootstrap;
import net.minecraft.world.level.block.Block;
import net.minecraft.world.level.block.FallingBlock;
import net.minecraft.world.level.block.state.BlockState;

public final class DumpBlocks26_1 {
    public static void main(String[] arguments) throws Exception {
        // Held before the game starts. Bootstrapping installs a logging
        // framework that takes System.out over, and a document written through
        // that arrives wrapped in log decoration and parses as nothing.
        PrintStream out = System.out;

        SharedConstants.tryDetectVersion();
        Bootstrap.bootStrap();
        // Touching Blocks runs the static block that fills BLOCK_STATE_REGISTRY
        // and calls initCache on every state, which is what computes the answer
        // being measured. Reading the registry without it finds it empty.
        Class.forName("net.minecraft.world.level.block.Blocks");

        Set<String> climbable = readTag("/data/minecraft/tags/block/climbable.json");

        List<String> entries = new ArrayList<String>();
        for (Block block : BuiltInRegistries.BLOCK) {
            List<BlockState> states = block.getStateDefinition().getPossibleStates();
            if (states.isEmpty()) {
                throw new IllegalStateException("block with no states: " + block);
            }

            int minState = Integer.MAX_VALUE;
            int maxState = Integer.MIN_VALUE;
            int blocking = 0;
            for (BlockState state : states) {
                int id = Block.BLOCK_STATE_REGISTRY.getId(state);
                if (id < 0) {
                    throw new IllegalStateException("unregistered state: " + state);
                }
                minState = Math.min(minState, id);
                maxState = Math.max(maxState, id);
                if (state.blocksMotion()) {
                    blocking++;
                }
            }
            // The document describes a block's states as a range, so a block
            // whose states are not contiguous would be described wrongly and
            // silently. Nothing in the game promises they are contiguous; this
            // checks rather than trusts.
            if (maxState - minState + 1 != states.size()) {
                throw new IllegalStateException("non-contiguous states for " + block);
            }

            // The block records whichever answer most of its states give, so
            // the exception list stays short and is empty for all but one
            // block in this version. The tie goes to blocking, which is the
            // reading that refuses to walk.
            boolean base = blocking * 2 >= states.size();
            StringBuilder exceptions = new StringBuilder();
            for (BlockState state : states) {
                boolean blocks = state.blocksMotion();
                if (blocks == base) {
                    continue;
                }
                if (exceptions.length() > 0) {
                    exceptions.append(',');
                }
                exceptions.append("{\"state\":")
                    .append(Block.BLOCK_STATE_REGISTRY.getId(state))
                    .append(",\"blocksMovement\":").append(blocks).append('}');
            }

            String key = BuiltInRegistries.BLOCK.getKey(block).toString();
            StringBuilder entry = new StringBuilder();
            entry.append("{\"id\":").append(BuiltInRegistries.BLOCK.getId(block))
                .append(",\"name\":\"").append(escape(key)).append('"')
                .append(",\"blocksMovement\":").append(base)
                .append(",\"falls\":").append(block instanceof FallingBlock)
                .append(",\"climbable\":").append(climbable.contains(key))
                .append(",\"stateRange\":{\"from\":").append(minState)
                .append(",\"to\":").append(maxState).append('}');
            if (exceptions.length() > 0) {
                entry.append(",\"stateExceptions\":[").append(exceptions).append(']');
            }
            entry.append('}');
            entries.add(entry.toString());
        }

        StringBuilder document = new StringBuilder();
        document.append("{\"stateEncoding\":\"block-state-registry\",\"blocks\":[");
        for (int index = 0; index < entries.size(); index++) {
            if (index > 0) {
                document.append(',');
            }
            document.append(entries.get(index));
        }
        document.append("]}");
        out.print(document);
        out.flush();
    }

    // readTag reads one block tag document out of the jar on the classpath.
    //
    // It refuses a nested tag reference instead of resolving one. The climbable
    // tag has none today, and a version that gave it one would silently lose
    // every block behind the reference if this quietly skipped it.
    private static Set<String> readTag(String resource) throws Exception {
        Set<String> values = new HashSet<String>();
        InputStream stream = DumpBlocks26_1.class.getResourceAsStream(resource);
        if (stream == null) {
            throw new IllegalStateException("no tag document at " + resource);
        }
        try {
            JsonObject document = JsonParser.parseReader(
                new InputStreamReader(stream, StandardCharsets.UTF_8)).getAsJsonObject();
            for (JsonElement value : document.getAsJsonArray("values")) {
                String name = value.getAsString();
                if (name.startsWith("#")) {
                    throw new IllegalStateException(resource + " references tag " + name);
                }
                values.add(name);
            }
        } finally {
            stream.close();
        }
        if (values.isEmpty()) {
            throw new IllegalStateException(resource + " describes nothing");
        }

        return values;
    }

    private static String escape(String value) {
        StringBuilder out = new StringBuilder();
        for (int index = 0; index < value.length(); index++) {
            char character = value.charAt(index);
            if (character == '"' || character == '\\') {
                out.append('\\');
            }
            out.append(character);
        }
        return out.toString();
    }
}
`
