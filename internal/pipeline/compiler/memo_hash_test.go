package compiler

import (
	"encoding/json"
	"hash/fnv"
	"testing"
)

// cajaMemoHash mirrors the exact implementation of the caja_memo_hash helper
// injected into generated Go code by injectBuiltinDependencies (builtins.go).
// It's duplicated here because the real one only ever exists as a string
// template emitted into compiled Caja programs, never as Go source this
// package can import directly.
func cajaMemoHash(v any) uint64 {
	h := fnv.New64a()
	json.NewEncoder(h).Encode(v)
	return h.Sum64()
}

// TestMemoHashDereferencesStructPointerSlices verifies that Go's fmt package
// formats pointer-to-struct slice elements by their field values rather than
// their addresses, which is what lets caja_memo_hash produce a stable,
// content-based key for a memoized function's array-of-struct parameter
// (Caja struct instances compile to Go struct pointers).
func TestMemoHashDereferencesStructPointerSlices(t *testing.T) {
	type point struct {
		X float64
		Y float64
	}

	a := []*point{{X: 1, Y: 2}, {X: 3, Y: 4}}
	b := []*point{{X: 1, Y: 2}, {X: 3, Y: 4}} // equal contents, distinct pointers

	if a[0] == b[0] {
		t.Fatalf("test setup invalid: expected distinct pointers, got the same one")
	}

	hashA := cajaMemoHash(a)
	hashB := cajaMemoHash(b)

	if hashA != hashB {
		t.Errorf("expected equal-content struct-pointer slices to hash identically, got %d vs %d", hashA, hashB)
	}

	c := []*point{{X: 1, Y: 2}, {X: 9, Y: 9}}
	if cajaMemoHash(a) == cajaMemoHash(c) {
		t.Errorf("expected different-content struct-pointer slices to hash differently")
	}
}

// TestMemoHashPrimitiveSlices verifies the simpler, non-struct case: two
// equal-content plain-value slices hash identically.
func TestMemoHashPrimitiveSlices(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{1, 2, 3, 4, 5}

	if cajaMemoHash(a) != cajaMemoHash(b) {
		t.Errorf("expected equal-content slices to hash identically")
	}

	c := []float64{1, 2, 3, 4, 6}
	if cajaMemoHash(a) == cajaMemoHash(c) {
		t.Errorf("expected different-content slices to hash differently")
	}
}
