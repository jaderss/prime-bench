package dev.jaderss.primebench.spring;

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
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.web.server.LocalServerPort;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;

@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
class GatewayIntegrationTest {

    private static final Map<String, String> BODIES = Map.of(
            "/accounts/acc-7",
            "{\"id\":\"acc-7\",\"owner\":\"Ada Example\",\"currency\":\"USD\",\"balanceMinor\":1250075}",
            "/accounts/acc-9",
            "{\"id\":\"acc-9\",\"currency\":\"USD\",\"balanceMinor\":100}",
            "/accounts/acc-9/transactions",
            "{\"accountId\":\"acc-9\"}",
            "/accounts/acc-7/transactions",
            "{\"accountId\":\"acc-7\",\"transactions\":[{\"id\":\"tx-1\",\"amountMinor\":-1250},"
                    + "{\"id\":\"tx-2\",\"amountMinor\":650000},{\"id\":\"tx-3\",\"amountMinor\":-750}]}",
            "/rates",
            "{\"base\":\"USD\",\"rates\":{\"EUR\":0.92}}");
    private static final HttpServer DOWNSTREAM = startDownstream();
    private static final HttpClient CLIENT = HttpClient.newHttpClient();

    @LocalServerPort
    private int port;

    @DynamicPropertySource
    static void downstreamUrl(DynamicPropertyRegistry registry) {
        registry.add(
                "downstream.url",
                () -> "http://localhost:" + DOWNSTREAM.getAddress().getPort());
    }

    @AfterAll
    static void stopDownstream() {
        DOWNSTREAM.stop(0);
    }

    @ParameterizedTest
    @CsvSource({
        "/aggregate?account=acc-7, 200",
        "/aggregate?account=acc-8, 502",
        "/aggregate?account=acc-9, 502",
        "/aggregate?account=acc%2F7, 400",
        "/aggregate, 400",
        "/stats, 200",
        "/actuator/health, 200"
    })
    void answersGetRequests(String path, int expected) throws Exception {
        assertThat(get(path).statusCode()).isEqualTo(expected);
    }

    @Test
    void aggregateReturnsTheCombinedJson() throws Exception {
        assertThat(get("/aggregate?account=acc-7").body())
                .contains("\"spentMinor\":2000", "\"receivedMinor\":650000", "\"balanceEurMinor\":1150069");
    }

    @Test
    void downstreamFailureHidesTheDetails() throws Exception {
        assertThat(get("/aggregate?account=acc-9").body())
                .contains("downstream unavailable")
                .doesNotContain("acc-9", "Exception", "http://");
    }

    private HttpResponse<String> get(String path) throws Exception {
        HttpRequest request = HttpRequest.newBuilder(URI.create("http://localhost:" + port + path))
                .build();
        return CLIENT.send(request, BodyHandlers.ofString());
    }

    private static HttpServer startDownstream() {
        try {
            HttpServer server = HttpServer.create(new InetSocketAddress("localhost", 0), 0);
            server.createContext("/", exchange -> {
                String body = BODIES.get(exchange.getRequestURI().getPath());
                byte[] bytes = body == null ? new byte[0] : body.getBytes(StandardCharsets.UTF_8);
                exchange.getResponseHeaders().set("Content-Type", "application/json");
                exchange.sendResponseHeaders(body == null ? 404 : 200, body == null ? -1 : bytes.length);
                try (var out = exchange.getResponseBody()) {
                    out.write(bytes);
                }
            });
            server.start();
            return server;
        } catch (IOException e) {
            throw new IllegalStateException(e);
        }
    }
}
