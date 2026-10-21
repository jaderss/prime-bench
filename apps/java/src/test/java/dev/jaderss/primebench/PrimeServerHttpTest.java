package dev.jaderss.primebench;

import static org.assertj.core.api.Assertions.assertThat;

import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpResponse.BodyHandlers;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;

class PrimeServerHttpTest {

    private static final HttpClient CLIENT = HttpClient.newHttpClient();
    private static HttpServer downstream;
    private static HttpServer server;

    @BeforeAll
    static void start() throws IOException {
        Map<String, String> bodies = Map.of(
                "/accounts/acc-7", GatewayTest.ACCOUNT,
                "/accounts/acc-7/transactions", GatewayTest.TRANSACTIONS,
                "/rates", GatewayTest.RATES);
        downstream = HttpServer.create(new InetSocketAddress("localhost", 0), 0);
        downstream.createContext("/", exchange -> {
            String body = bodies.get(exchange.getRequestURI().getPath());
            byte[] bytes = body == null ? new byte[0] : body.getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(body == null ? 404 : 200, body == null ? -1 : bytes.length);
            try (var out = exchange.getResponseBody()) {
                out.write(bytes);
            }
        });
        downstream.start();
        String downstreamUrl = "http://localhost:" + downstream.getAddress().getPort();
        server = PrimeServer.start(new Config("server", 0, 64, 1, 1, 5, "test", "results", downstreamUrl));
    }

    @AfterAll
    static void stop() {
        server.stop(0);
        downstream.stop(0);
    }

    @ParameterizedTest
    @CsvSource({
        "/prime, 200",
        "/prime?bits=128, 200",
        "/prime?bits=1, 400",
        "/aggregate?account=acc-7, 200",
        "/aggregate?account=acc-8, 502",
        "/aggregate?account=acc%2F7, 400",
        "/aggregate, 400",
        "/stats, 200",
        "/health, 200"
    })
    void answersGetRequests(String path, int expected) throws Exception {
        assertThat(send("GET", path).statusCode()).isEqualTo(expected);
    }

    @Test
    void rejectsOtherMethods() throws Exception {
        assertThat(send("POST", "/prime").statusCode()).isEqualTo(405);
    }

    @Test
    void aggregateReturnsTheCombinedJson() throws Exception {
        HttpResponse<String> response = send("GET", "/aggregate?account=acc-7");

        assertThat(Gateway.JSON.readValue(response.body(), Gateway.Aggregate.class))
                .isEqualTo(GatewayTest.EXPECTED);
    }

    private static HttpResponse<String> send(String method, String path) throws Exception {
        URI uri = URI.create("http://localhost:" + server.getAddress().getPort() + path);
        HttpRequest request = HttpRequest.newBuilder(uri)
                .method(method, HttpRequest.BodyPublishers.noBody())
                .build();
        return CLIENT.send(request, BodyHandlers.ofString());
    }
}
