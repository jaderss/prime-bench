package dev.jaderss.primebench;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.concurrent.ExecutionException;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;

class GatewayTest {

    static final String ACCOUNT =
            "{\"id\":\"acc-7\",\"owner\":\"Ada Example\",\"currency\":\"USD\",\"balanceMinor\":1250075,\"status\":\"active\"}";
    static final String TRANSACTIONS = "{\"accountId\":\"acc-7\",\"transactions\":["
            + "{\"id\":\"tx-1\",\"amountMinor\":-1250,\"category\":\"groceries\"},"
            + "{\"id\":\"tx-2\",\"amountMinor\":650000,\"category\":\"salary\"},"
            + "{\"id\":\"tx-3\",\"amountMinor\":-750,\"category\":\"transport\"}]}";
    static final String RATES = "{\"base\":\"USD\",\"rates\":{\"EUR\":0.92,\"GBP\":0.79}}";
    static final Gateway.Aggregate EXPECTED = new Gateway.Aggregate("acc-7", "USD", 1250075, 3, 2000, 650000, 11500.69);

    @Test
    void summarizeTotalsSpentReceivedAndConvertsBalance() {
        Gateway.Aggregate aggregate = Gateway.summarize(
                Gateway.JSON.readValue(ACCOUNT, Gateway.Account.class),
                Gateway.JSON.readValue(TRANSACTIONS, Gateway.TransactionList.class),
                Gateway.JSON.readValue(RATES, Gateway.Rates.class));

        assertThat(aggregate).isEqualTo(EXPECTED);
    }

    @Test
    void aggregateCallsAllThreeDownstreamEndpoints() throws Exception {
        HttpServer downstream = stub(Map.of(
                "/accounts/acc-7", ACCOUNT,
                "/accounts/acc-7/transactions", TRANSACTIONS,
                "/rates", RATES));
        try {
            assertThat(new Gateway(url(downstream)).aggregate("acc-7")).isEqualTo(EXPECTED);
        } finally {
            downstream.stop(0);
        }
    }

    @Test
    void aggregateFailsWhenADownstreamCallFails() throws IOException {
        HttpServer downstream = stub(Map.of("/rates", RATES));
        try {
            Gateway gateway = new Gateway(url(downstream));

            assertThatThrownBy(() -> gateway.aggregate("acc-7"))
                    .isInstanceOf(ExecutionException.class)
                    .hasMessageContaining("answered 404");
        } finally {
            downstream.stop(0);
        }
    }

    @ParameterizedTest
    @CsvSource({"acc-7, true", "ACC-123, true", "acc/7, false", "../rates, false", "acc 7, false", "'', false"})
    void validAccountId(String id, boolean expected) {
        assertThat(Gateway.validAccountId(id)).isEqualTo(expected);
    }

    private static HttpServer stub(Map<String, String> bodies) throws IOException {
        HttpServer server = HttpServer.create(new InetSocketAddress("localhost", 0), 0);
        server.createContext("/", exchange -> {
            String body = bodies.get(exchange.getRequestURI().getPath());
            byte[] bytes = body == null ? new byte[0] : body.getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(body == null ? 404 : 200, body == null ? -1 : bytes.length);
            try (var out = exchange.getResponseBody()) {
                out.write(bytes);
            }
        });
        server.start();
        return server;
    }

    private static String url(HttpServer server) {
        return "http://localhost:" + server.getAddress().getPort();
    }
}
