package physics

// dump18Source is the reflective dumper compiled and run against a prepared
// 1.8.9 server jar. Every identifier it names comes from
// reference/notes/physics-symbols-1.8.9.md.
//
// It lives in a Go file rather than a committed .java file because the
// repository tracks no .java at all, so that decompiled output can never be
// committed by accident. See TestRestrictedReferenceArtifactsAreNotTracked.
const dump18Source = `import java.lang.reflect.Field;
import java.util.Map;
import java.util.TreeMap;

public final class Dump1_8 {
    public static void main(String[] arguments) throws Exception {
        bootstrap();

        Map<String, Float> slipperiness = new TreeMap<String, Float>();
        Class<?> blockClass = Class.forName("net.minecraft.block.Block");
        Field registryField = blockClass.getDeclaredField("blockRegistry");
        registryField.setAccessible(true);
        Object registry = registryField.get(null);
        Field slipperinessField = blockClass.getDeclaredField("slipperiness");
        slipperinessField.setAccessible(true);

        java.lang.reflect.Method nameOf = registry.getClass()
            .getMethod("getNameForObject", Object.class);
        nameOf.setAccessible(true);
        for (Object block : (Iterable<?>) registry) {
            if (block == null) {
                continue;
            }
            Object name = nameOf.invoke(registry, block);
            if (name == null) {
                continue;
            }
            slipperiness.put(shortName(name.toString()), slipperinessField.getFloat(block));
        }

        Field sinField = Class.forName("net.minecraft.util.MathHelper")
            .getDeclaredField("SIN_TABLE");
        sinField.setAccessible(true);
        float[] sinTable = (float[]) sinField.get(null);

        StringBuilder out = new StringBuilder();
        out.append("{\"defaultSlipperiness\":0.6,\"blockSlipperiness\":{");
        boolean first = true;
        for (Map.Entry<String, Float> entry : slipperiness.entrySet()) {
            if (!first) {
                out.append(',');
            }
            first = false;
            out.append('"').append(entry.getKey()).append("\":")
               .append(Float.toString(entry.getValue()));
        }
        out.append("},\"sinTableBase64\":\"")
           .append(encode(sinTable)).append("\"}");
        System.out.print(out);
    }

    private static void bootstrap() throws Exception {
        Class.forName("net.minecraft.init.Bootstrap").getMethod("register").invoke(null);
    }

    private static String shortName(String key) {
        int colon = key.indexOf(':');
        return colon < 0 ? key : key.substring(colon + 1);
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
