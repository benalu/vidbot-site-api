package keyvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// Vault menyimpan dan mengambil data terenkripsi.
// Enkripsi pakai AES-256-GCM dengan key dari MASTER_KEY / KEY_VAULT_SECRET.
//
// Cara kerja:
// - saat key dibuat: plain key → encrypt → simpan di Redis sebagai "keyvault:{keyHash}"
// - saat admin minta lihat: ambil dari Redis → decrypt → tampilkan plain key
// - enkripsi hanya bisa dibuka oleh siapapun yang punya secret (yaitu admin)
type Vault struct {
	encKey []byte // 32 bytes AES-256 key
}

var Default *Vault

// Init inisialisasi vault dengan secret string.
// Secret di-hash SHA-256 untuk menghasilkan 32-byte AES key.
func Init(secret string) {
	if secret == "" {
		// Kalau tidak dikonfigurasi, vault tidak aktif
		// CreateKey akan tetap jalan tapi plain key tidak disimpan
		Default = nil
		return
	}
	h := sha256.Sum256([]byte(secret))
	Default = &Vault{encKey: h[:]}
}

// IsReady cek apakah vault aktif (secret sudah dikonfigurasi)
func IsReady() bool {
	return Default != nil
}

// Encrypt mengenkripsi plain text dan mengembalikan hex string.
// Format output: nonce(24 hex chars) + ciphertext(hex)
func (v *Vault) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(v.encKey)
	if err != nil {
		return "", fmt.Errorf("keyvault: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("keyvault: create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("keyvault: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt mendekripsi hex string yang dihasilkan oleh Encrypt.
func (v *Vault) Decrypt(encrypted string) (string, error) {
	data, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("keyvault: decode hex: %w", err)
	}

	block, err := aes.NewCipher(v.encKey)
	if err != nil {
		return "", fmt.Errorf("keyvault: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("keyvault: create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("keyvault: ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("keyvault: decrypt failed: %w", err)
	}

	return string(plaintext), nil
}

// StoreKey menyimpan plain key ke Redis vault.
// Redis key: "keyvault:{keyHash}"
// Dipanggil sekali saat key dibuat.
func StoreKey(keyHash, plainKey string, redisSetter func(key, val string) error) error {
	if Default == nil {
		return nil // vault tidak aktif, skip silently
	}
	encrypted, err := Default.Encrypt(plainKey)
	if err != nil {
		return err
	}
	return redisSetter("keyvault:"+keyHash, encrypted)
}

// RetrieveKey mengambil dan mendekripsi plain key dari Redis vault.
// Dipanggil oleh admin endpoint saja.
func RetrieveKey(keyHash string, redisGetter func(key string) (string, error)) (string, error) {
	if Default == nil {
		return "", fmt.Errorf("key vault not configured — set KEY_VAULT_SECRET in .env")
	}
	encrypted, err := redisGetter("keyvault:" + keyHash)
	if err != nil {
		return "", fmt.Errorf("key not found in vault (key was created before vault was enabled)")
	}
	return Default.Decrypt(encrypted)
}
