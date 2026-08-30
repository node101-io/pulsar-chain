package types

// ProofEntry is the bounded internal representation used by validator-side
// commitment construction. It is not consensus state by itself.
type ProofEntry struct {
	Key    ProofKey
	Record ProofRecord
}
