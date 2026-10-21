package dev.jaderss.primebench;

import static java.math.BigInteger.ONE;
import static org.assertj.core.api.Assertions.assertThat;

import java.math.BigInteger;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;
import org.junit.jupiter.params.provider.ValueSource;

class PrimeGeneratorTest {

    private final PrimeGenerator generator = new PrimeGenerator(20);

    static BigInteger mersenne(int exponent) {
        return ONE.shiftLeft(exponent).subtract(ONE);
    }

    static BigInteger[] primes() {
        return new BigInteger[] {
            BigInteger.valueOf(2),
            BigInteger.valueOf(3),
            BigInteger.valueOf(997),
            BigInteger.valueOf(7919),
            mersenne(61),
            mersenne(127),
            mersenne(521)
        };
    }

    static BigInteger[] composites() {
        return new BigInteger[] {
            BigInteger.ZERO,
            ONE,
            BigInteger.valueOf(4),
            BigInteger.valueOf(561),
            BigInteger.valueOf(41041),
            BigInteger.valueOf(825265),
            BigInteger.valueOf(3215031751L),
            mersenne(67),
            mersenne(61).multiply(mersenne(127))
        };
    }

    @ParameterizedTest
    @MethodSource("primes")
    void acceptsKnownPrimes(BigInteger n) {
        assertThat(generator.isProbablePrime(n)).isTrue();
    }

    @ParameterizedTest
    @MethodSource("composites")
    void rejectsKnownComposites(BigInteger n) {
        assertThat(generator.isProbablePrime(n)).isFalse();
    }

    @ParameterizedTest
    @ValueSource(ints = {64, 256, 1024})
    void generatesPrimeWithExactBitLength(int bits) {
        BigInteger prime = generator.probablePrime(bits);

        assertThat(prime.bitLength()).isEqualTo(bits);
        assertThat(prime.isProbablePrime(50)).isTrue();
    }
}
