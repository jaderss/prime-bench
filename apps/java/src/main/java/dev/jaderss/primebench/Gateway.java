package dev.jaderss.primebench;

import static tools.jackson.databind.DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpResponse.BodyHandlers;
import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.regex.Pattern;
import tools.jackson.databind.json.JsonMapper;

final class Gateway {

    static final JsonMapper JSON =
            JsonMapper.builder().disable(FAIL_ON_UNKNOWN_PROPERTIES).build();

    private static final Duration TIMEOUT = Duration.ofSeconds(5);
    private static final Pattern ACCOUNT_ID = Pattern.compile("[A-Za-z0-9-]{1,64}");

    record Account(String id, String currency, long balanceMinor) {}

    record Transaction(long amountMinor) {}

    record TransactionList(List<Transaction> transactions) {}

    record Rates(Map<String, Double> rates) {}

    record Aggregate(
            String accountId,
            String currency,
            long balanceMinor,
            int transactions,
            long spentMinor,
            long receivedMinor,
            double balanceEur) {}

    private final HttpClient client;
    private final URI base;

    Gateway(String baseUrl) {
        this.client = HttpClient.newBuilder()
                .version(HttpClient.Version.HTTP_1_1)
                .connectTimeout(TIMEOUT)
                .build();
        this.base = URI.create(baseUrl);
    }

    Aggregate aggregate(String accountId) throws InterruptedException, ExecutionException {
        try (var calls = Executors.newVirtualThreadPerTaskExecutor()) {
            Future<Account> account = calls.submit(() -> get("/accounts/" + accountId, Account.class));
            Future<TransactionList> transactions =
                    calls.submit(() -> get("/accounts/" + accountId + "/transactions", TransactionList.class));
            Future<Rates> rates = calls.submit(() -> get("/rates", Rates.class));
            return summarize(account.get(), transactions.get(), rates.get());
        }
    }

    static Aggregate summarize(Account account, TransactionList list, Rates rates) {
        long spent = 0;
        long received = 0;
        for (Transaction transaction : list.transactions()) {
            if (transaction.amountMinor() < 0) {
                spent -= transaction.amountMinor();
            } else {
                received += transaction.amountMinor();
            }
        }
        double balanceEur = Math.round(account.balanceMinor() * rates.rates().getOrDefault("EUR", 0.0)) / 100.0;
        return new Aggregate(
                account.id(),
                account.currency(),
                account.balanceMinor(),
                list.transactions().size(),
                spent,
                received,
                balanceEur);
    }

    static boolean validAccountId(String id) {
        return ACCOUNT_ID.matcher(id).matches();
    }

    private <T> T get(String path, Class<T> type) throws IOException, InterruptedException {
        HttpRequest request = HttpRequest.newBuilder(base.resolve(path))
                .timeout(TIMEOUT)
                .GET()
                .build();
        HttpResponse<byte[]> response = client.send(request, BodyHandlers.ofByteArray());
        if (response.statusCode() != 200) {
            throw new IOException("%s answered %d".formatted(path, response.statusCode()));
        }
        return JSON.readValue(response.body(), type);
    }
}
