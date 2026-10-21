package main

import (
	"math/big"
	"testing"
)

func mersenne(exponent uint) *big.Int {
	n := new(big.Int).Lsh(one, exponent)
	return n.Sub(n, one)
}

func TestKnownPrimes(t *testing.T) {
	g := newGenerator(20)
	for _, n := range []*big.Int{
		big.NewInt(2), big.NewInt(3), big.NewInt(997), big.NewInt(7919),
		mersenne(61), mersenne(127), mersenne(521),
	} {
		if !g.isProbablePrime(n) {
			t.Errorf("%v should be prime", n)
		}
	}
}

func TestKnownComposites(t *testing.T) {
	g := newGenerator(20)
	for _, n := range []*big.Int{
		big.NewInt(0), big.NewInt(1), big.NewInt(4), big.NewInt(561), big.NewInt(41041),
		big.NewInt(825265), big.NewInt(3215031751), mersenne(67), new(big.Int).Mul(mersenne(61), mersenne(127)),
	} {
		if g.isProbablePrime(n) {
			t.Errorf("%v should be composite", n)
		}
	}
}

func TestGeneratedPrimeHasExactBitLength(t *testing.T) {
	g := newGenerator(20)
	for _, bits := range []int{64, 256, 1024} {
		p := g.probablePrime(bits)
		if p.BitLen() != bits || !p.ProbablyPrime(20) {
			t.Errorf("bits=%d got %v (len %d)", bits, p, p.BitLen())
		}
	}
}
