package dev.jaderss.primebench.spring;

import dev.jaderss.primebench.spring.Downstream.Account;
import dev.jaderss.primebench.spring.Downstream.Rates;
import dev.jaderss.primebench.spring.Downstream.Transaction;
import dev.jaderss.primebench.spring.Downstream.TransactionList;
import java.util.concurrent.CompletableFuture;
import org.springframework.core.task.AsyncTaskExecutor;
import org.springframework.stereotype.Service;

@Service
class AggregateService {

    record Aggregate(
            String accountId,
            String currency,
            long balanceMinor,
            int transactions,
            long spentMinor,
            long receivedMinor,
            long balanceEurMinor) {}

    private final DownstreamClient downstream;
    private final AsyncTaskExecutor executor;

    AggregateService(DownstreamClient downstream, AsyncTaskExecutor applicationTaskExecutor) {
        this.downstream = downstream;
        this.executor = applicationTaskExecutor;
    }

    Aggregate aggregate(String accountId) {
        CompletableFuture<Account> account =
                CompletableFuture.supplyAsync(() -> downstream.account(accountId), executor);
        CompletableFuture<TransactionList> transactions =
                CompletableFuture.supplyAsync(() -> downstream.transactions(accountId), executor);
        CompletableFuture<Rates> rates = CompletableFuture.supplyAsync(downstream::rates, executor);
        return CompletableFuture.allOf(account, transactions, rates)
                .thenApply(done -> combine(account.join(), transactions.join(), rates.join()))
                .join();
    }

    static Aggregate combine(Account account, TransactionList list, Rates rates) {
        long spent = list.transactions().stream()
                .mapToLong(Transaction::amountMinor)
                .filter(amount -> amount < 0)
                .map(Math::negateExact)
                .sum();
        long received = list.transactions().stream()
                .mapToLong(Transaction::amountMinor)
                .filter(amount -> amount >= 0)
                .sum();
        double eurRate = rates.rates().getOrDefault("EUR", 0.0);
        return new Aggregate(
                account.id(),
                account.currency(),
                account.balanceMinor(),
                list.transactions().size(),
                spent,
                received,
                Math.round(account.balanceMinor() * eurRate));
    }
}
