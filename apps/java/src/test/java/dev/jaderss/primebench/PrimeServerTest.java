package dev.jaderss.primebench;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

class PrimeServerTest {

    @Test
    void usesDefaultBitsWithoutQuery() {
        assertThat(PrimeServer.parseBits(null, 1024)).contains(1024);
    }

    @Test
    void acceptsBitsInRange() {
        assertThat(PrimeServer.parseBits("bits=2048", 1024)).contains(2048);
    }

    @Test
    void rejectsBitsOutOfRangeOrInvalid() {
        assertThat(PrimeServer.parseBits("bits=32", 1024)).isEmpty();
        assertThat(PrimeServer.parseBits("bits=9000", 1024)).isEmpty();
        assertThat(PrimeServer.parseBits("bits=abc", 1024)).isEmpty();
    }
}
