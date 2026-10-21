package dev.jaderss.primebench;

import static java.util.stream.Collectors.joining;

record RunInfo(String mode, String runId, int bits, int concurrency) {

    static final String LANG = "java";

    static String langVersion() {
        return Runtime.version().version().stream().map(String::valueOf).collect(joining("."));
    }

    static int cpus() {
        return Runtime.getRuntime().availableProcessors();
    }
}
