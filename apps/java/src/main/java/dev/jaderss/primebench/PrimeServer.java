package dev.jaderss.primebench;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.Locale;
import java.util.Optional;
import java.util.concurrent.Executors;

final class PrimeServer {

    private static final int MIN_BITS = 64;
    private static final int MAX_BITS = 8192;
    private static final int BACKLOG = 4096;

    private PrimeServer() {}

    static void run(Config config) throws IOException {
        var generator = new PrimeGenerator(config.rounds());
        var server = HttpServer.create(new InetSocketAddress(config.port()), BACKLOG);
        server.setExecutor(Executors.newVirtualThreadPerTaskExecutor());
        server.createContext("/prime", exchange -> prime(exchange, generator, config.bits()));
        server.createContext("/stats", PrimeServer::stats);
        server.createContext("/health", exchange -> respond(exchange, 200, "text/plain", "ok"));
        server.start();
        System.out.printf("java %s listening on :%d%n", RunInfo.langVersion(), config.port());
    }

    private static void prime(HttpExchange exchange, PrimeGenerator generator, int defaultBits) throws IOException {
        Optional<Integer> bits = parseBits(exchange.getRequestURI().getQuery(), defaultBits);
        if (bits.isEmpty()) {
            respond(
                    exchange,
                    400,
                    "text/plain",
                    "bits must be an integer between %d and %d".formatted(MIN_BITS, MAX_BITS));
            return;
        }
        long start = System.nanoTime();
        String prime = generator.probablePrime(bits.get()).toString();
        double ms = (System.nanoTime() - start) / 1e6;
        respond(
                exchange,
                200,
                "application/json",
                String.format(Locale.ROOT, "{\"bits\":%d,\"prime\":\"%s\",\"ms\":%.3f}", bits.get(), prime, ms));
    }

    private static void stats(HttpExchange exchange) throws IOException {
        ProcessStats stats = ProcessStats.read();
        respond(
                exchange,
                200,
                "application/json",
                String.format(
                        Locale.ROOT,
                        "{\"lang\":\"%s\",\"version\":\"%s\",\"cpus\":%d,\"rss_mb\":%.3f,\"cpu_seconds\":%.3f}",
                        RunInfo.LANG,
                        RunInfo.langVersion(),
                        RunInfo.cpus(),
                        stats.rssMb(),
                        stats.cpuSeconds()));
    }

    static Optional<Integer> parseBits(String query, int defaultBits) {
        if (query == null || !query.startsWith("bits=")) {
            return Optional.of(defaultBits);
        }
        try {
            int bits = Integer.parseInt(query.substring("bits=".length()));
            return bits >= MIN_BITS && bits <= MAX_BITS ? Optional.of(bits) : Optional.empty();
        } catch (NumberFormatException e) {
            return Optional.empty();
        }
    }

    private static void respond(HttpExchange exchange, int status, String contentType, String body) throws IOException {
        byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
        exchange.getResponseHeaders().set("Content-Type", contentType);
        exchange.sendResponseHeaders(status, bytes.length);
        try (var out = exchange.getResponseBody()) {
            out.write(bytes);
        }
    }
}
