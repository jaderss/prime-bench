package main

import (
	crand "crypto/rand"
	"math/big"
	"math/rand/v2"
)

var (
	one         = big.NewInt(1)
	two         = big.NewInt(2)
	three       = big.NewInt(3)
	smallPrimes = sieve(1000)
)

type generator struct {
	rng    *rand.ChaCha8
	rounds int
}

func newGenerator(rounds int) *generator {
	var seed [32]byte
	crand.Read(seed[:])
	return &generator{rng: rand.NewChaCha8(seed), rounds: rounds}
}

func (g *generator) probablePrime(bits int) *big.Int {
	for {
		candidate := g.randomBits(bits)
		candidate.SetBit(candidate, bits-1, 1)
		candidate.SetBit(candidate, 0, 1)
		if g.isProbablePrime(candidate) {
			return candidate
		}
	}
}

func (g *generator) isProbablePrime(n *big.Int) bool {
	if n.Cmp(two) < 0 {
		return false
	}
	remainder := new(big.Int)
	for _, p := range smallPrimes {
		if n.Cmp(p) == 0 {
			return true
		}
		if remainder.Mod(n, p).Sign() == 0 {
			return false
		}
	}
	return g.millerRabin(n)
}

func (g *generator) millerRabin(n *big.Int) bool {
	nMinusOne := new(big.Int).Sub(n, one)
	nMinusThree := new(big.Int).Sub(n, three)
	s := nMinusOne.TrailingZeroBits()
	d := new(big.Int).Rsh(nMinusOne, s)
	x := new(big.Int)
	for range g.rounds {
		base := g.randomBits(n.BitLen())
		base.Mod(base, nMinusThree).Add(base, two)
		x.Exp(base, d, n)
		if x.Cmp(one) == 0 || x.Cmp(nMinusOne) == 0 {
			continue
		}
		if !squaresToMinusOne(x, s, n, nMinusOne) {
			return false
		}
	}
	return true
}

func squaresToMinusOne(x *big.Int, s uint, n, nMinusOne *big.Int) bool {
	for range s - 1 {
		x.Mul(x, x).Mod(x, n)
		if x.Cmp(nMinusOne) == 0 {
			return true
		}
	}
	return false
}

func (g *generator) randomBits(bits int) *big.Int {
	buf := make([]byte, (bits+7)/8)
	g.rng.Read(buf)
	n := new(big.Int).SetBytes(buf)
	return n.Rsh(n, uint(len(buf)*8-bits))
}

func sieve(limit int) []*big.Int {
	composite := make([]bool, limit)
	var primes []*big.Int
	for i := 2; i < limit; i++ {
		if composite[i] {
			continue
		}
		primes = append(primes, big.NewInt(int64(i)))
		for j := i * i; j < limit; j += i {
			composite[j] = true
		}
	}
	return primes
}
