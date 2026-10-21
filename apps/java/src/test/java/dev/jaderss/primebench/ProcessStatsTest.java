package dev.jaderss.primebench;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.within;

import org.junit.jupiter.api.Test;

class ProcessStatsTest {

    static final String STAT =
            "4242 (my (odd) proc) S 1 4242 4242 0 -1 4194560 2000 0 0 0 250 50 0 0 20 0 9 0 1000 123456 789 0\n";
    static final String STATUS = "Name:\tjava\nVmPeak:\t  999999 kB\nVmRSS:\t  204800 kB\nThreads:\t9\n";

    @Test
    void cpuSecondsAddsUserAndSystemTicksAfterTheLastParenthesis() {
        assertThat(ProcessStats.parseCpuSeconds(STAT)).isCloseTo(3.0, within(1e-9));
    }

    @Test
    void rssReadsVmRssInMegabytes() {
        assertThat(ProcessStats.parseRssMb(STATUS)).isCloseTo(200.0, within(1e-9));
    }

    @Test
    void rssIsZeroWithoutVmRss() {
        assertThat(ProcessStats.parseRssMb("Name:\tkthreadd\n")).isZero();
    }
}
