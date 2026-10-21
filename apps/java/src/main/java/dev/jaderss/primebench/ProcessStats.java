package dev.jaderss.primebench;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

record ProcessStats(double rssMb, double cpuSeconds) {

    private static final Path STATUS = Path.of("/proc/self/status");
    private static final Path STAT = Path.of("/proc/self/stat");
    private static final double CLOCK_TICKS_PER_SECOND = 100;

    static ProcessStats read() {
        return new ProcessStats(readRssMb(), readCpuSeconds());
    }

    private static double readRssMb() {
        try (var lines = Files.lines(STATUS)) {
            return lines.filter(line -> line.startsWith("VmRSS:"))
                    .map(line -> Double.parseDouble(line.split("\\s+")[1]) / 1024)
                    .findFirst()
                    .orElse(0.0);
        } catch (IOException e) {
            return 0;
        }
    }

    private static double readCpuSeconds() {
        try {
            String stat = Files.readString(STAT);
            String[] fields = stat.substring(stat.lastIndexOf(')') + 2).split(" ");
            return (Double.parseDouble(fields[11]) + Double.parseDouble(fields[12])) / CLOCK_TICKS_PER_SECOND;
        } catch (IOException e) {
            return 0;
        }
    }
}
