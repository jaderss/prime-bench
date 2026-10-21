package dev.jaderss.primebench.spring;

import static org.assertj.core.api.Assertions.assertThat;

import dev.jaderss.primebench.spring.AggregateService.Aggregate;
import dev.jaderss.primebench.spring.Downstream.Account;
import dev.jaderss.primebench.spring.Downstream.Rates;
import dev.jaderss.primebench.spring.Downstream.Transaction;
import dev.jaderss.primebench.spring.Downstream.TransactionList;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class AggregateServiceTest {

    @Test
    void combineTotalsSpentReceivedAndConvertsBalance() {
        Aggregate aggregate = AggregateService.combine(
                new Account("acc-7", "USD", 1250075),
                new TransactionList(List.of(new Transaction(-1250), new Transaction(650000), new Transaction(-750))),
                new Rates(Map.of("EUR", 0.92, "GBP", 0.79)));

        assertThat(aggregate).isEqualTo(new Aggregate("acc-7", "USD", 1250075, 3, 2000, 650000, 1150069));
    }

    @Test
    void missingEuroRateGivesZero() {
        Aggregate aggregate = AggregateService.combine(
                new Account("acc-7", "USD", 100), new TransactionList(List.of()), new Rates(Map.of()));

        assertThat(aggregate.balanceEurMinor()).isZero();
    }
}
