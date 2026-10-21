package dev.jaderss.primebench;

import java.util.Optional;

record Config(
        String mode,
        int port,
        int bits,
        int count,
        int concurrency,
        int rounds,
        String runId,
        String resultsDir,
        String downstreamUrl) {

    static Config fromEnv() {
        return new Config(
                env("MODE", "server"),
                envInt("PORT", 8080),
                envInt("BITS", 1024),
                envInt("COUNT", 200),
                envInt("CONCURRENCY", Runtime.getRuntime().availableProcessors()),
                envInt("ROUNDS", 20),
                env("RUN_ID", "local"),
                env("RESULTS_DIR", "results"),
                env("DOWNSTREAM_URL", "http://mock:8080"));
    }

    private static String env(String key, String fallback) {
        return Optional.ofNullable(System.getenv(key))
                .filter(value -> !value.isBlank())
                .orElse(fallback);
    }

    private static int envInt(String key, int fallback) {
        try {
            int value = Integer.parseInt(env(key, ""));
            return value > 0 ? value : fallback;
        } catch (NumberFormatException e) {
            return fallback;
        }
    }
}
