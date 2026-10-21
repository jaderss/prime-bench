package dev.jaderss.primebench;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.locks.ReentrantLock;

final class Recorder {

    record Snapshot(List<Double> latenciesMs, int errors) {}

    private final ReentrantLock lock = new ReentrantLock();
    private final List<Double> all = new ArrayList<>();
    private List<Double> interval = new ArrayList<>();
    private int intervalErrors;
    private int allErrors;

    void record(Duration latency) {
        double ms = latency.toNanos() / 1e6;
        lock.lock();
        try {
            interval.add(ms);
            all.add(ms);
        } finally {
            lock.unlock();
        }
    }

    Snapshot drain() {
        lock.lock();
        try {
            Snapshot snapshot = new Snapshot(interval, intervalErrors);
            interval = new ArrayList<>();
            intervalErrors = 0;
            return snapshot;
        } finally {
            lock.unlock();
        }
    }

    Snapshot total() {
        lock.lock();
        try {
            return new Snapshot(List.copyOf(all), allErrors);
        } finally {
            lock.unlock();
        }
    }
}
