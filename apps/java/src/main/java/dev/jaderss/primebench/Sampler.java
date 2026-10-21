package dev.jaderss.primebench;

import java.io.BufferedWriter;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Locale;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

final class Sampler implements AutoCloseable {

    static final String HEADER = "kind,lang,lang_version,mode,run_id,elapsed_s,cpus,bits,concurrency,completed,errors,"
            + "throughput_ops_s,latency_p50_ms,latency_p95_ms,latency_p99_ms,latency_max_ms,rss_mb,cpu_pct";

    record Row(
            String kind,
            double elapsedS,
            List<Double> latenciesMs,
            int errors,
            double wallS,
            double rssMb,
            double cpuPct) {

        List<Double> sorted() {
            return latenciesMs.stream().sorted().toList();
        }

        double throughput() {
            return latenciesMs.size() / wallS;
        }

        String summary() {
            List<Double> sorted = sorted();
            return String.format(
                    Locale.ROOT,
                    "completed=%d errors=%d throughput=%.2f/s p50=%.2fms p99=%.2fms peak_rss=%.1fMB cpu=%.0f%%",
                    sorted.size(),
                    errors,
                    throughput(),
                    percentile(sorted, 50),
                    percentile(sorted, 99),
                    rssMb,
                    cpuPct);
        }
    }

    private static final long MIN_INTERVAL_NANOS = 250_000_000L;

    private final RunInfo info;
    private final Recorder recorder;
    private final BufferedWriter writer;
    private final ScheduledExecutorService ticker = Executors.newSingleThreadScheduledExecutor();
    private final long startNanos;
    private final double startCpu;
    private long lastNanos;
    private double lastCpu;
    private double peakRss;

    Sampler(Path dir, RunInfo info, Recorder recorder) throws IOException {
        this.info = info;
        this.recorder = recorder;
        Files.createDirectories(dir);
        this.writer =
                Files.newBufferedWriter(dir.resolve("%s_%s_%s.csv".formatted(info.runId(), info.mode(), RunInfo.LANG)));
        this.startNanos = System.nanoTime();
        this.lastNanos = startNanos;
        this.startCpu = ProcessStats.read().cpuSeconds();
        this.lastCpu = startCpu;
        writeLine(HEADER);
        ticker.scheduleAtFixedRate(this::sample, 1, 1, TimeUnit.SECONDS);
    }

    private synchronized void sample() {
        long now = System.nanoTime();
        ProcessStats stats = ProcessStats.read();
        Recorder.Snapshot snapshot = recorder.drain();
        double wall = (now - lastNanos) / 1e9;
        peakRss = Math.max(peakRss, stats.rssMb());
        write(new Row(
                "interval",
                (now - startNanos) / 1e9,
                snapshot.latenciesMs(),
                snapshot.errors(),
                wall,
                stats.rssMb(),
                (stats.cpuSeconds() - lastCpu) / wall * 100));
        lastNanos = now;
        lastCpu = stats.cpuSeconds();
    }

    Row finish() {
        ticker.close();
        if (System.nanoTime() - lastNanos >= MIN_INTERVAL_NANOS) {
            sample();
        }
        long now = System.nanoTime();
        ProcessStats stats = ProcessStats.read();
        Recorder.Snapshot total = recorder.total();
        double wall = (now - startNanos) / 1e9;
        Row summary = new Row(
                "summary",
                wall,
                total.latenciesMs(),
                total.errors(),
                wall,
                Math.max(peakRss, stats.rssMb()),
                (stats.cpuSeconds() - startCpu) / wall * 100);
        write(summary);
        return summary;
    }

    @Override
    public void close() throws IOException {
        writer.close();
    }

    static double percentile(List<Double> sorted, double p) {
        if (sorted.isEmpty()) {
            return 0;
        }
        int rank = (int) Math.ceil(p / 100 * sorted.size());
        return sorted.get(Math.max(rank, 1) - 1);
    }

    private void write(Row row) {
        List<Double> sorted = row.sorted();
        writeLine(String.join(
                ",",
                row.kind(),
                RunInfo.LANG,
                RunInfo.langVersion(),
                info.mode(),
                info.runId(),
                format(row.elapsedS()),
                String.valueOf(RunInfo.cpus()),
                String.valueOf(info.bits()),
                String.valueOf(info.concurrency()),
                String.valueOf(sorted.size()),
                String.valueOf(row.errors()),
                format(row.throughput()),
                format(percentile(sorted, 50)),
                format(percentile(sorted, 95)),
                format(percentile(sorted, 99)),
                format(percentile(sorted, 100)),
                format(row.rssMb()),
                format(row.cpuPct())));
    }

    private void writeLine(String line) {
        try {
            writer.write(line);
            writer.newLine();
            writer.flush();
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }
    }

    private static String format(double value) {
        return String.format(Locale.ROOT, "%.3f", value);
    }
}
