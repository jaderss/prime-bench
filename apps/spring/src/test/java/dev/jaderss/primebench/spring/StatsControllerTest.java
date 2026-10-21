package dev.jaderss.primebench.spring;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.within;

import java.util.List;
import org.junit.jupiter.api.Test;

class StatsControllerTest {

    @Test
    void cpuSecondsAddsUserAndSystemTicksAfterTheLastParenthesis() {
        String stat = "4242 (my (odd) proc) S 1 4242 4242 0 -1 4194560 2000 0 0 0 250 50 0 0 20 0 9 0 1000 1 2 0";

        assertThat(StatsController.cpuSeconds(stat)).isCloseTo(3.0, within(1e-9));
    }

    @Test
    void residentMegabytesReadsVmRss() {
        assertThat(StatsController.residentMegabytes(List.of("Name:\tjava", "VmRSS:\t  204800 kB")))
                .isCloseTo(200.0, within(1e-9));
        assertThat(StatsController.residentMegabytes(List.of("Name:\tkthreadd")))
                .isZero();
    }
}
