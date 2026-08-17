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
const dumpBlocks261Source = `import java.io.PrintStream;
import java.util.ArrayList;
import java.util.List;
import net.minecraft.SharedConstants;
import net.minecraft.core.registries.BuiltInRegistries;
import net.minecraft.server.Bootstrap;
import net.minecraft.world.level.block.Block;
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

            StringBuilder entry = new StringBuilder();
            entry.append("{\"id\":").append(BuiltInRegistries.BLOCK.getId(block))
                .append(",\"name\":\"").append(escape(BuiltInRegistries.BLOCK.getKey(block).toString())).append('"')
                .append(",\"blocksMovement\":").append(base)
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
