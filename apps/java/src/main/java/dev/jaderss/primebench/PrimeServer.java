package dev.jaderss.primebench;

import static java.lang.System.Logger.Level.WARNING;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.Locale;
import java.util.Optional;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Executors;

final class PrimeServer {

    private static final int MIN_BITS = 64;
    private static final int MAX_BITS = 8192;
    private static final int BACKLOG = 4096;
    private static final String DOWNSTREAM_UNAVAILABLE = "downstream unavailable";
    private static final System.Logger LOG = System.getLogger(PrimeServer.class.getName());

    private PrimeServer() {}

    static void run(Config config) throws IOException {
        HttpServer server = start(config);
        System.out.printf(
                "java %s listening on :%d%n",
                RunInfo.langVersion(), server.getAddress().getPort());
    }

    static HttpServer start(Config config) throws IOException {
        var generator = new PrimeGenerator(config.rounds());
        var server = HttpServer.create(new InetSocketAddress(config.port()), BACKLOG);
        server.setExecutor(Executors.newVirtualThreadPerTaskExecutor());
        server.createContext("/prime", get(exchange -> prime(exchange, generator, config.bits())));
        var gateway = new Gateway(config.downstreamUrl());
        server.createContext("/aggregate", get(exchange -> aggregate(exchange, gateway)));
        server.createContext("/stats", get(PrimeServer::stats));
        server.createContext("/health", get(exchange -> respond(exchange, 200, "text/plain", "ok")));
        server.start();
        return server;
    }

    private static HttpHandler get(HttpHandler handler) {
        return exchange -> {
            if ("GET".equals(exchange.getRequestMethod())) {
                handler.handle(exchange);
            } else {
                respond(exchange, 405, "text/plain", "method not allowed");
            }
        };
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

    private static void aggregate(HttpExchange exchange, Gateway gateway) throws IOException {
        Optional<String> accountId =
                queryParam(exchange.getRequestURI().getQuery(), "account").filter(Gateway::validAccountId);
        if (accountId.isEmpty()) {
            respond(exchange, 400, "text/plain", "account must be 1 to 64 letters, digits or dashes");
            return;
        }
        try {
            String body = Gateway.JSON.writeValueAsString(gateway.aggregate(accountId.get()));
            respond(exchange, 200, "application/json", body);
        } catch (ExecutionException e) {
            LOG.log(WARNING, "downstream call failed: {0}", describe(e.getCause()));
            respond(exchange, 502, "text/plain", DOWNSTREAM_UNAVAILABLE);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            respond(exchange, 502, "text/plain", DOWNSTREAM_UNAVAILABLE);
        }
    }

    private static String describe(Throwable cause) {
        return cause instanceof IOException && cause.getMessage() != null
                ? cause.getMessage()
                : cause.getClass().getSimpleName();
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
        Optional<String> raw = queryParam(query, "bits");
        if (raw.isEmpty()) {
            return Optional.of(defaultBits);
        }
        try {
            int bits = Integer.parseInt(raw.get());
            return bits >= MIN_BITS && bits <= MAX_BITS ? Optional.of(bits) : Optional.empty();
        } catch (NumberFormatException e) {
            return Optional.empty();
        }
    }

    private static Optional<String> queryParam(String query, String name) {
        if (query == null) {
            return Optional.empty();
        }
        return Arrays.stream(query.split("&"))
                .map(pair -> pair.split("=", 2))
                .filter(pair -> pair[0].equals(name))
                .map(pair -> pair.length > 1 ? pair[1] : "")
                .findFirst();
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
