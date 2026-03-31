package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"sync"
	"time"
)

// SRP-6a implementation using RFC 5054 Appendix A 2048-bit group parameters.
// The SRP username is a fixed application constant shared by client and server.
const srpUsername = "yinmonote"

// srpN is the 2048-bit safe prime from RFC 5054 Appendix A.
// srpG is the generator (2) for the same group.
// srpk is the SRP-6a multiplier: k = SHA256(pad256(N) || pad256(g)).
var (
	srpN *big.Int
	srpG *big.Int
	srpk *big.Int
)

func init() {
	// RFC 5054 Appendix A, 2048-bit group.
	nHex := "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1" +
		"29024E088A67CC74020BBEA63B139B22514A08798E3404DD" +
		"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245" +
		"E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED" +
		"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D" +
		"C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F" +
		"83655D23DCA3AD961C62F356208552BB9ED529077096966D" +
		"670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B" +
		"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9" +
		"DE2BCBF6955817183995497CEA956AE515D2261898FA0510" +
		"15728E5A8AACAA68FFFFFFFFFFFFFFFF"
	var ok bool
	srpN, ok = new(big.Int).SetString(nHex, 16)
	if !ok {
		panic("SRP: failed to parse N constant")
	}
	srpG = big.NewInt(2)

	// k = SHA256(pad256(N) || pad256(g))
	h := sha256.New()
	h.Write(pad256(srpN))
	h.Write(pad256(srpG))
	srpk = new(big.Int).SetBytes(h.Sum(nil))
}

// pad256 returns the big-endian byte representation of n, left-padded with
// zeros to exactly 256 bytes. This is required for all SRP hash inputs so that
// the byte sequences are unambiguous regardless of leading zero bits.
func pad256(n *big.Int) []byte {
	b := n.Bytes()
	if len(b) >= 256 {
		// Truncate to last 256 bytes only if longer (should never happen for N-group values).
		return b[len(b)-256:]
	}
	out := make([]byte, 256)
	copy(out[256-len(b):], b)
	return out
}

// srpComputeVerifier computes the SRP verifier v = g^x mod N and returns it as
// a 512-character lowercase hex string (256 bytes big-endian).
// srpSaltBytes is the raw 16-byte random salt; password is the plaintext password.
func srpComputeVerifier(srpSaltBytes []byte, password string) string {
	x := srpComputeX(srpSaltBytes, password)
	v := new(big.Int).Exp(srpG, x, srpN)
	return hex.EncodeToString(pad256(v))
}

// srpComputeX computes x = SHA256(salt || SHA256("yinmonote:password")).
// This follows the RFC 2945 / RFC 5054 definition: x = H(s | H(I | ":" | P)).
func srpComputeX(saltBytes []byte, password string) *big.Int {
	inner := sha256.Sum256([]byte(srpUsername + ":" + password))
	h := sha256.New()
	h.Write(saltBytes)
	h.Write(inner[:])
	return new(big.Int).SetBytes(h.Sum(nil))
}

// srpSession holds ephemeral server-side state for one in-progress SRP handshake.
type srpSession struct {
	A        *big.Int  // client ephemeral public key (received in /srp/init)
	b        *big.Int  // server ephemeral private key (generated in /srp/init)
	B        *big.Int  // server ephemeral public key (sent in /srp/init response)
	S        *big.Int  // pre-computed session key (shared secret)
	expireAt time.Time // handshake TTL (5 minutes)
}

var (
	srpSessions   = make(map[string]*srpSession) // keyed by pad256(A) hex
	srpSessionsMu sync.Mutex
)

func init() {
	// Evict expired SRP handshake sessions every 2 minutes to prevent map growth.
	go func() {
		for {
			time.Sleep(2 * time.Minute)
			now := time.Now()
			srpSessionsMu.Lock()
			for key, sess := range srpSessions {
				if now.After(sess.expireAt) {
					delete(srpSessions, key)
				}
			}
			srpSessionsMu.Unlock()
		}
	}()
}

// srpInitHandshake processes the client's A value and returns B_hex.
// It validates A != 0 mod N, computes B = (k*v + g^b) mod N, pre-computes
// the shared secret S, and stores the session keyed by pad256(A) hex.
// verifierHex is the stored SRP verifier (512 hex chars).
func srpInitHandshake(aHex string, verifierHex string) (string, error) {
	aBytes, err := hex.DecodeString(aHex)
	if err != nil || len(aBytes) == 0 {
		return "", errSRPInvalidA
	}
	A := new(big.Int).SetBytes(aBytes)

	// Security: reject A = 0 mod N (would allow attacker to bypass password check).
	if new(big.Int).Mod(A, srpN).Sign() == 0 {
		return "", errSRPInvalidA
	}

	vBytes, err := hex.DecodeString(verifierHex)
	if err != nil || len(vBytes) == 0 {
		return "", errSRPInvalidVerifier
	}
	v := new(big.Int).SetBytes(vBytes)

	zero := new(big.Int)
	var B, bPriv *big.Int

	// Generate b and compute B = (k*v + g^b) mod N.
	// Retry in the astronomically-unlikely event that B = 0 mod N.
	for {
		bBytes := make([]byte, 32)
		if _, err := rand.Read(bBytes); err != nil {
			return "", err
		}
		bPriv = new(big.Int).SetBytes(bBytes)

		kv := new(big.Int).Mul(srpk, v)
		kv.Mod(kv, srpN)
		gb := new(big.Int).Exp(srpG, bPriv, srpN)
		B = new(big.Int).Add(kv, gb)
		B.Mod(B, srpN)

		if new(big.Int).Mod(B, srpN).Cmp(zero) != 0 {
			break
		}
	}

	// u = SHA256(pad256(A) || pad256(B))
	uh := sha256.New()
	uh.Write(pad256(A))
	uh.Write(pad256(B))
	u := new(big.Int).SetBytes(uh.Sum(nil))

	// S = (A * v^u) ^ b mod N
	vu := new(big.Int).Exp(v, u, srpN)
	Avu := new(big.Int).Mul(A, vu)
	Avu.Mod(Avu, srpN)
	S := new(big.Int).Exp(Avu, bPriv, srpN)

	// Store session keyed by the canonical (pad256) form of A.
	// Limit map size to 200 to prevent unauthenticated callers from exhausting
	// memory by flooding /srp/init with distinct A values.
	aKeyHex := hex.EncodeToString(pad256(A))
	srpSessionsMu.Lock()
	if len(srpSessions) >= 200 {
		srpSessionsMu.Unlock()
		return "", errSRPInvalidA
	}
	srpSessions[aKeyHex] = &srpSession{
		A:        A,
		b:        bPriv,
		B:        B,
		S:        S,
		expireAt: time.Now().Add(5 * time.Minute),
	}
	srpSessionsMu.Unlock()

	return hex.EncodeToString(pad256(B)), nil
}

// srpVerifyHandshake validates the client's M1, consumes the session (one-time use),
// and returns (M2_hex, bearerToken, error).
// M1 is verified with constant-time comparison to prevent timing side-channels.
func srpVerifyHandshake(aHex string, m1Hex string) (string, string, error) {
	// Normalise A to the canonical pad256 form used as the session map key.
	aBytes, err := hex.DecodeString(aHex)
	if err != nil || len(aBytes) == 0 {
		return "", "", errSRPSessionNotFound
	}
	a := new(big.Int).SetBytes(aBytes)
	aKeyHex := hex.EncodeToString(pad256(a))

	srpSessionsMu.Lock()
	sess, exists := srpSessions[aKeyHex]
	if exists {
		delete(srpSessions, aKeyHex) // consume immediately — one-time use
	}
	srpSessionsMu.Unlock()

	if !exists {
		return "", "", errSRPSessionNotFound
	}
	if time.Now().After(sess.expireAt) {
		return "", "", errSRPSessionExpired
	}

	// Expected M1 = SHA256(pad256(A) || pad256(B) || pad256(S))
	m1h := sha256.New()
	m1h.Write(pad256(sess.A))
	m1h.Write(pad256(sess.B))
	m1h.Write(pad256(sess.S))
	expectedM1 := m1h.Sum(nil)

	gotM1, err := hex.DecodeString(m1Hex)
	if err != nil || subtle.ConstantTimeCompare(expectedM1, gotM1) != 1 {
		return "", "", errSRPBadM1
	}

	// M2 = SHA256(pad256(A) || M1_bytes || pad256(S))
	m2h := sha256.New()
	m2h.Write(pad256(sess.A))
	m2h.Write(expectedM1)
	m2h.Write(pad256(sess.S))
	m2 := m2h.Sum(nil)

	// Issue a cryptographically-random Bearer token (32 bytes → ~43 base64 chars).
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	return hex.EncodeToString(m2), token, nil
}

// Sentinel errors returned by SRP handshake functions.
var (
	errSRPInvalidA        = srpError("invalid_A")
	errSRPInvalidVerifier = srpError("invalid_verifier")
	errSRPSessionNotFound = srpError("session_not_found")
	errSRPSessionExpired  = srpError("session_expired")
	errSRPBadM1           = srpError("bad_M1")
)

type srpError string

func (e srpError) Error() string { return string(e) }

// srpNewSalt generates a fresh 16-byte random SRP salt and returns it as Base64.
func srpNewSalt() (string, []byte, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	return base64.StdEncoding.EncodeToString(b), b, nil
}
