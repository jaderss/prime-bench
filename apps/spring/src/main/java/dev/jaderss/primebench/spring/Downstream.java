package dev.jaderss.primebench.spring;

import java.util.List;
import java.util.Map;

final class Downstream {

    private Downstream() {}

    record Account(String id, String currency, long balanceMinor) {}

    record Transaction(long amountMinor) {}

    record TransactionList(List<Transaction> transactions) {}

    record Rates(Map<String, Double> rates) {}
}
