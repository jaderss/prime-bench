package dev.jaderss.primebench;

import static java.math.BigInteger.ONE;
import static java.math.BigInteger.TWO;

import java.math.BigInteger;
import java.util.List;
import java.util.Random;
import java.util.concurrent.ThreadLocalRandom;
import java.util.stream.IntStream;

final class PrimeGenerator {

    private static final BigInteger THREE = BigInteger.valueOf(3);
    private static final List<BigInteger> SMALL_PRIMES = sieve(1000);

    private final int rounds;

    PrimeGenerator(int rounds) {
        this.rounds = rounds;
    }

    BigInteger probablePrime(int bits) {
        Random random = ThreadLocalRandom.current();
        while (true) {
            BigInteger candidate = new BigInteger(bits, random).setBit(bits - 1).setBit(0);
            if (isProbablePrime(candidate)) {
                return candidate;
            }
        }
    }

    boolean isProbablePrime(BigInteger n) {
        if (n.compareTo(TWO) < 0) {
            return false;
        }
        for (BigInteger p : SMALL_PRIMES) {
            if (n.equals(p)) {
                return true;
            }
            if (n.mod(p).signum() == 0) {
                return false;
            }
        }
        return millerRabin(n);
    }

    private boolean millerRabin(BigInteger n) {
        Random random = ThreadLocalRandom.current();
        BigInteger nMinusOne = n.subtract(ONE);
        BigInteger nMinusThree = n.subtract(THREE);
        int s = nMinusOne.getLowestSetBit();
        BigInteger d = nMinusOne.shiftRight(s);
        for (int round = 0; round < rounds; round++) {
            BigInteger base =
                    new BigInteger(n.bitLength(), random).mod(nMinusThree).add(TWO);
            BigInteger x = base.modPow(d, n);
            if (!x.equals(ONE) && !x.equals(nMinusOne) && !squaresToMinusOne(x, s, n, nMinusOne)) {
                return false;
            }
        }
        return true;
    }

    private static boolean squaresToMinusOne(BigInteger x, int s, BigInteger n, BigInteger nMinusOne) {
        for (int i = 1; i < s; i++) {
            x = x.multiply(x).mod(n);
            if (x.equals(nMinusOne)) {
                return true;
            }
        }
        return false;
    }

    private static List<BigInteger> sieve(int limit) {
        boolean[] composite = new boolean[limit];
        for (int i = 2; i * i < limit; i++) {
            if (!composite[i]) {
                for (int j = i * i; j < limit; j += i) {
                    composite[j] = true;
                }
            }
        }
        return IntStream.range(2, limit)
                .filter(i -> !composite[i])
                .mapToObj(BigInteger::valueOf)
                .toList();
    }
}
