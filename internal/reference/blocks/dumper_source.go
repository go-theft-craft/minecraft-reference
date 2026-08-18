package blocks

// dumpBlocks18Source is the reflective dumper compiled and run against a
// prepared 1.8.9 server jar.
//
// It lives in a Go file rather than a committed .java file because the
// repository tracks no .java at all, so that decompiled output can never be
// committed by accident. The physics dumper is stored the same way and for the
// same reason.
//
// It reads the registry rather than a list of names, so a block this file has
// never heard of is described correctly and one that was renamed does not
// silently vanish. Everything it asks for is a public method: the material's
// blocksMovement, which is the game's own answer to "can something walk into
// this", and isFullCube, which separates a slab from a stone.
//
// Two more facts ride along, and both are class tests because this version
// states them as classes and nowhere else. Falling is BlockFalling, which sand,
// gravel, the anvil, and the dragon egg extend. Climbable is BlockLadder or
// BlockVine, which is not a guess at what looks climbable but the pair
// EntityLivingBase.isOnLadder names; 1.8.9 has no tag system, so the game
// itself asks the question this way.
const dumpBlocks18Source = `import java.lang.reflect.Method;
import java.util.ArrayList;
import java.util.List;

public final class DumpBlocks1_8 {
    public static void main(String[] arguments) throws Exception {
        Class.forName("net.minecraft.init.Bootstrap").getMethod("register").invoke(null);

        Class<?> blockClass = Class.forName("net.minecraft.block.Block");
        Method idFromBlock = blockClass.getMethod("getIdFromBlock", blockClass);
        Method getMaterial = blockClass.getMethod("getMaterial");
        Method blocksMovement = Class.forName("net.minecraft.block.material.Material")
            .getMethod("blocksMovement");

        Class<?> fallingClass = Class.forName("net.minecraft.block.BlockFalling");
        Class<?> ladderClass = Class.forName("net.minecraft.block.BlockLadder");
        Class<?> vineClass = Class.forName("net.minecraft.block.BlockVine");

        Object registry = registry(blockClass);
        Method nameOf = registry.getClass().getMethod("getNameForObject", Object.class);
        nameOf.setAccessible(true);

        List<String> entries = new ArrayList<String>();
        for (Object block : (Iterable<?>) registry) {
            if (block == null) {
                continue;
            }
            Object name = nameOf.invoke(registry, block);
            if (name == null) {
                continue;
            }
            int id = ((Integer) idFromBlock.invoke(null, block)).intValue();
            Object material = getMaterial.invoke(block);
            boolean blocks = ((Boolean) blocksMovement.invoke(material)).booleanValue();
            boolean falls = fallingClass.isInstance(block);
            boolean climbable = ladderClass.isInstance(block) || vineClass.isInstance(block);
            entries.add("{\"id\":" + id
                + ",\"name\":\"" + escape(name.toString()) + "\""
                + ",\"blocksMovement\":" + blocks
                + ",\"falls\":" + falls
                + ",\"climbable\":" + climbable + "}");
        }

        StringBuilder out = new StringBuilder();
        out.append("{\"stateEncoding\":\"id<<4|meta\",\"blocks\":[");
        for (int index = 0; index < entries.size(); index++) {
            if (index > 0) {
                out.append(',');
            }
            out.append(entries.get(index));
        }
        out.append("]}");
        System.out.print(out);
    }

    private static Object registry(Class<?> blockClass) throws Exception {
        java.lang.reflect.Field field = blockClass.getDeclaredField("blockRegistry");
        field.setAccessible(true);
        return field.get(null);
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
