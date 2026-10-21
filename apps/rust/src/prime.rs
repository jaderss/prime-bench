use std::sync::LazyLock;

use num_bigint::BigUint;
use num_traits::{One, ToPrimitive, Zero};
use rand::Rng;

static SMALL_PRIMES: LazyLock<Vec<u32>> = LazyLock::new(|| sieve(1000));

#[derive(Clone, Copy)]
pub struct Generator {
    rounds: u32,
}

impl Generator {
    pub fn new(rounds: u32) -> Self {
        Self { rounds }
    }

    pub fn probable_prime(&self, bits: u64) -> BigUint {
        let mut rng = rand::rng();
        loop {
            let mut candidate = random_bits(&mut rng, bits);
            candidate.set_bit(bits - 1, true);
            candidate.set_bit(0, true);
            if self.is_probable_prime(&candidate) {
                return candidate;
            }
        }
    }

    pub fn is_probable_prime(&self, n: &BigUint) -> bool {
        if n.to_u32().is_some_and(|small| small < 2) {
            return false;
        }
        for &p in SMALL_PRIMES.iter() {
            if n.to_u32() == Some(p) {
                return true;
            }
            if (n % p).is_zero() {
                return false;
            }
        }
        self.miller_rabin(n)
    }

    fn miller_rabin(&self, n: &BigUint) -> bool {
        let mut rng = rand::rng();
        let one = BigUint::one();
        let n_minus_one = n - 1u32;
        let n_minus_three = n - 3u32;
        let s = n_minus_one.trailing_zeros().unwrap_or(0);
        let d = &n_minus_one >> s;
        (0..self.rounds).all(|_| {
            let base = random_bits(&mut rng, n.bits()) % &n_minus_three + 2u32;
            let x = base.modpow(&d, n);
            x == one || x == n_minus_one || squares_to_minus_one(x, s, n, &n_minus_one)
        })
    }
}

fn squares_to_minus_one(mut x: BigUint, s: u64, n: &BigUint, n_minus_one: &BigUint) -> bool {
    for _ in 1..s {
        x = &x * &x % n;
        if x == *n_minus_one {
            return true;
        }
    }
    false
}

fn random_bits(rng: &mut impl Rng, bits: u64) -> BigUint {
    let mut buf = vec![0u8; bits.div_ceil(8) as usize];
    rng.fill_bytes(&mut buf);
    BigUint::from_bytes_be(&buf) >> (buf.len() as u64 * 8 - bits)
}

fn sieve(limit: usize) -> Vec<u32> {
    let mut composite = vec![false; limit];
    let mut primes = Vec::new();
    for i in 2..limit {
        if composite[i] {
            continue;
        }
        primes.push(i as u32);
        for j in (i * i..limit).step_by(i) {
            composite[j] = true;
        }
    }
    primes
}

#[cfg(test)]
mod tests {
    use super::*;

    fn mersenne(exponent: u32) -> BigUint {
        (BigUint::one() << exponent) - 1u32
    }

    #[test]
    fn accepts_known_primes() {
        let generator = Generator::new(20);
        for n in [2u32, 3, 997, 7919].map(BigUint::from).into_iter().chain([
            mersenne(61),
            mersenne(127),
            mersenne(521),
        ]) {
            assert!(generator.is_probable_prime(&n), "{n} should be prime");
        }
    }

    #[test]
    fn rejects_known_composites() {
        let generator = Generator::new(20);
        let small = [0u64, 1, 4, 561, 41041, 825265, 3215031751].map(BigUint::from);
        for n in small
            .into_iter()
            .chain([mersenne(67), mersenne(61) * mersenne(127)])
        {
            assert!(!generator.is_probable_prime(&n), "{n} should be composite");
        }
    }

    #[test]
    fn generates_prime_with_exact_bit_length() {
        let generator = Generator::new(20);
        for bits in [64, 256, 1024] {
            let prime = generator.probable_prime(bits);
            assert_eq!(prime.bits(), bits);
            assert!(generator.is_probable_prime(&prime));
        }
    }
}
