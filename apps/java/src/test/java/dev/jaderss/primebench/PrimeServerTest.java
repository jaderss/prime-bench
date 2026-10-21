package dev.jaderss.primebench;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.NullSource;
import org.junit.jupiter.params.provider.ValueSource;

class PrimeServerTest {

    @ParameterizedTest
    @NullSource
    @ValueSource(strings = {"", "x=1", "bitsx=2048"})
    void usesDefaultBitsWhenAbsent(String query) {
        assertThat(PrimeServer.parseBits(query, 1024)).contains(1024);
    }

    @ParameterizedTest
    @CsvSource({"bits=2048, 2048", "x=1&bits=2048, 2048", "bits=2048&x=1, 2048", "bits=64, 64", "bits=8192, 8192"})
    void readsBitsAnywhereInQuery(String query, int expected) {
        assertThat(PrimeServer.parseBits(query, 1024)).contains(expected);
    }

    @ParameterizedTest
    @ValueSource(strings = {"bits=", "bits", "bits=32", "bits=9000", "bits=abc"})
    void rejectsEmptyOutOfRangeOrInvalidBits(String query) {
        assertThat(PrimeServer.parseBits(query, 1024)).isEmpty();
    }
}
