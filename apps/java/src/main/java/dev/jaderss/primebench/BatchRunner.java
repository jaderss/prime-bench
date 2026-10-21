package dev.jaderss.primebench;

import java.io.IOException;
import java.nio.file.Path;
import java.time.Duration;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicInteger;

final class BatchRunner {

    private BatchRunner() {}

    static void run(Config config) throws IOException {
        var recorder = new Recorder();
        var info = new RunInfo("batch", config.runId(), config.bits(), config.concurrency());
        var remaining = new AtomicInteger(config.count());
        var generator = new PrimeGenerator(config.rounds());
        try (var sampler = new Sampler(Path.of(config.resultsDir()), info, recorder)) {
            try (var workers = Executors.newFixedThreadPool(config.concurrency())) {
                for (int i = 0; i < config.concurrency(); i++) {
                    workers.submit(() -> {
                        while (remaining.decrementAndGet() >= 0) {
                            long start = System.nanoTime();
                            generator.probablePrime(config.bits());
                            recorder.record(Duration.ofNanos(System.nanoTime() - start));
                        }
                    });
                }
            }
            Sampler.Row summary = sampler.finish();
            System.out.printf(
                    "java %s batch bits=%d count=%d workers=%d %s%n",
                    RunInfo.langVersion(), config.bits(), config.count(), config.concurrency(), summary.summary());
        }
    }
}
