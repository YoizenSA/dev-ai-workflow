package control

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// VAPIDKeys holds VAPID authentication keys.
type VAPIDKeys struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

// PushSender sends Web Push notifications.
type PushSender struct {
	store *PushStore
	vapid *VAPIDKeys
}

// NewPushSender creates a PushSender, loading or generating VAPID keys.
func NewPushSender(store *PushStore) (*PushSender, error) {
	vapid, err := loadOrGenerateVAPID()
	if err != nil {
		return nil, fmt.Errorf("vapid setup: %w", err)
	}
	return &PushSender{store: store, vapid: vapid}, nil
}

// PublicKey returns the VAPID public key as a base64url string.
func (ps *PushSender) PublicKey() string {
	return ps.vapid.PublicKey
}

// Send sends a notification to all subscriptions with the given title and body.
func (ps *PushSender) Send(title, body string) error {
	subs := ps.store.List()
	if len(subs) == 0 {
		return nil
	}
	payload := fmt.Sprintf(`{"title":%q,"body":%q}`, title, body)
	payloadBytes := []byte(payload)
	for _, sub := range subs {
		if err := ps.sendToSubscription(sub, payloadBytes); err != nil {
			// Gone or not found → subscription is dead
			if strings.Contains(err.Error(), "410") || strings.Contains(err.Error(), "404") {
				_ = ps.store.Unsubscribe(sub.Endpoint)
			}
		}
	}
	return nil
}

func (ps *PushSender) sendToSubscription(sub PushSubscription, payload []byte) error {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("salt: %w", err)
	}

	ephemeral, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("ephemeral: %w", err)
	}

	// Decode subscription keys (RFC 8291)
	p256dh, err := base64.RawURLEncoding.DecodeString(sub.Keys.P256DH)
	if err != nil {
		return fmt.Errorf("decode p256dh: %w", err)
	}
	auth, err := base64.RawURLEncoding.DecodeString(sub.Keys.Auth)
	if err != nil {
		return fmt.Errorf("decode auth: %w", err)
	}

	subPubX, subPubY := elliptic.Unmarshal(elliptic.P256(), p256dh)
	if subPubX == nil {
		return fmt.Errorf("invalid subscription public key")
	}

	// ECDH: shared secret = ephemeral.D * subPub
	sharedX, _ := elliptic.P256().ScalarMult(subPubX, subPubY, ephemeral.D.Bytes())
	if sharedX == nil {
		return fmt.Errorf("ecdh: nil shared secret")
	}

	// RFC 8291 §3.1: derive PRK (pseudo-random key)
	prk := hmacSHA256(auth, sharedX.Bytes())

	// info = "WebPush: info\x00" || subPub || ephemPub
	ephemPub := elliptic.Marshal(elliptic.P256(), ephemeral.PublicKey.X, ephemeral.PublicKey.Y)
	var info bytes.Buffer
	info.WriteString("WebPush: info\x00")
	info.Write(p256dh)
	info.Write(ephemPub)

	ikm := hkdfExpand(prk, info.Bytes(), 32)

	// Derive CEK and nonce
	prk2 := hmacSHA256(salt, ikm)
	cek := hkdfExpand(prk2, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(prk2, []byte("Content-Encoding: nonce\x00"), 12)

	// Encrypt padded payload with AES-128-GCM
	padded := append([]byte{0x00}, payload...) // minimal padding byte per spec

	block, err := aes.NewCipher(cek)
	if err != nil {
		return fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("gcm: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, padded, nil)

	// Build aes128gcm binary body
	var body bytes.Buffer
	body.Write(salt)                                     // 16 bytes salt
	_ = binary.Write(&body, binary.BigEndian, uint32(4096)) // record size
	body.WriteByte(byte(len(ephemPub)))                   // key length = 65
	body.Write(ephemPub)                                 // uncompressed public key
	body.Write(ciphertext)                               // encrypted payload + auth tag

	// Build VAPID JWT with endpoint origin as audience
	vapidJWT, err := ps.buildVAPIDJWT(sub.Endpoint)
	if err != nil {
		return fmt.Errorf("vapid jwt: %w", err)
	}

	authHeader := "vapid t=" + vapidJWT + ", k=" + strings.TrimRight(ps.vapid.PublicKey, "=")

	req, err := http.NewRequest("POST", sub.Endpoint, &body)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", "86400")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Encoding", "aes128gcm")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 204 && resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("push service returned %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// buildVAPIDJWT creates a signed JWT for VAPID per RFC 8292.
func (ps *PushSender) buildVAPIDJWT(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	origin := u.Scheme + "://" + u.Host

	header := `{"typ":"JWT","alg":"ES256"}`
	now := time.Now().Unix()
	payload := fmt.Sprintf(`{"aud":%q,"exp":%d,"sub":"mailto:admin@ywai.dev"}`, origin, now+86400)

	hB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	pB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signingInput := hB64 + "." + pB64

	// Reconstruct private key from stored bytes (pad to 32 bytes)
	privBytes, err := base64.RawURLEncoding.DecodeString(ps.vapid.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}

	// Pad to 32 bytes (P-256 scalar)
	scalar := make([]byte, 32)
	copy(scalar[32-len(privBytes):], privBytes)

	priv := new(ecdsa.PrivateKey)
	priv.Curve = elliptic.P256()
	priv.D = new(big.Int).SetBytes(scalar)
	priv.PublicKey.X, priv.PublicKey.Y = priv.Curve.ScalarBaseMult(scalar)

	hash := sha256.Sum256([]byte(signingInput))
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	// Encode signature as r||s, each 32 bytes, total 64
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	sig := make([]byte, 64)
	copy(sig[32-len(rBytes):], rBytes)
	copy(sig[64-len(sBytes):], sBytes)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return hB64 + "." + pB64 + "." + sigB64, nil
}

// loadOrGenerateVAPID reads VAPID keys from ~/.ywai/vapid-keys.json or generates new ones.
func loadOrGenerateVAPID() (*VAPIDKeys, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".ywai")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "vapid-keys.json")

	data, err := os.ReadFile(path)
	if err == nil {
		var keys VAPIDKeys
		if json.Unmarshal(data, &keys) == nil && keys.PrivateKey != "" && keys.PublicKey != "" {
			return &keys, nil
		}
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	privBytes := priv.D.Bytes()
	// Pad to exactly 32 bytes
	scalar := make([]byte, 32)
	copy(scalar[32-len(privBytes):], privBytes)

	pubBytes := elliptic.Marshal(priv.PublicKey.Curve, priv.PublicKey.X, priv.PublicKey.Y)

	keys := &VAPIDKeys{
		PrivateKey: base64.RawURLEncoding.EncodeToString(scalar),
		PublicKey:  base64.RawURLEncoding.EncodeToString(pubBytes),
	}

	out, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return nil, err
	}
	return keys, nil
}

// --- HKDF helpers (RFC 5869 subset) ---

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func hkdfExpand(prk, info []byte, length int) []byte {
	// Single iteration covers up to 32 bytes
	mac := hmac.New(sha256.New, prk)
	mac.Write(info)
	mac.Write([]byte{0x01})
	out := mac.Sum(nil)
	return out[:length]
}
