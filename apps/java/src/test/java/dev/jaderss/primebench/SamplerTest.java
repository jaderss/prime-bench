package dev.jaderss.primebench;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;

class SamplerTest {

    private static final List<Double> ONE_TO_TEN = List.of(1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0);

    @ParameterizedTest
    @CsvSource({"50, 5", "95, 10", "99, 10", "100, 10", "10, 1"})
    void percentileUsesNearestRank(double p, double expected) {
        assertThat(Sampler.percentile(ONE_TO_TEN, p)).isEqualTo(expected);
    }

    @Test
    void percentileOfEmptyListIsZero() {
        assertThat(Sampler.percentile(List.of(), 50)).isZero();
    }
}
