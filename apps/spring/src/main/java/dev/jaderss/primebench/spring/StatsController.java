package dev.jaderss.primebench.spring;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import org.springframework.boot.SpringBootVersion;
import org.springframework.core.NativeDetector;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
class StatsController {

    private static final double KILOBYTES_PER_MEGABYTE = 1024;
    private static final double TICKS_PER_SECOND = 100;
    private static final Path STATUS = Path.of("/proc/self/status");
    private static final Path STAT = Path.of("/proc/self/stat");

    @GetMapping("/stats")
    Map<String, Object> stats() {
        boolean linux = Files.isReadable(STAT);
        return Map.of(
                "lang", NativeDetector.inNativeImage() ? "spring-native" : "spring",
                "version",
                        SpringBootVersion.getVersion() + "-java"
                                + Runtime.version().feature(),
                "cpus", Runtime.getRuntime().availableProcessors(),
                "rss_mb", linux ? residentMegabytes(readLines(STATUS)) : 0.0,
                "cpu_seconds", linux ? cpuSeconds(String.join(" ", readLines(STAT))) : 0.0);
    }

    static double residentMegabytes(List<String> status) {
        for (String line : status) {
            String[] parts = line.trim().split("\\s+");
            if (parts[0].equals("VmRSS:")) {
                return Long.parseLong(parts[1]) / KILOBYTES_PER_MEGABYTE;
            }
        }
        return 0;
    }

    static double cpuSeconds(String stat) {
        List<String> afterName =
                List.of(stat.substring(stat.lastIndexOf(')') + 1).trim().split(" "));
        return (Long.parseLong(afterName.get(11)) + Long.parseLong(afterName.get(12))) / TICKS_PER_SECOND;
    }

    private static List<String> readLines(Path path) {
        try {
            return Files.readAllLines(path);
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }
    }
}
