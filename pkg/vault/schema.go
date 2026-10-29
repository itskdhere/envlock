package vault

import "encoding/json"

// VaultVersion represents the current vault envelope schema version.
const VaultVersion = 1

// CipherAES256GCM is the cipher algorithm used for payload encryption.
const CipherAES256GCM = "AES-256-GCM"

// RecipientEntry stores the wrapped DEK for a specific recipient public key.
type RecipientEntry struct {
	PubKey          string `json:"pubkey"`
	EphemeralPubKey string `json:"ephemeral_pubkey"`
	WrappedDEK      string `json:"wrapped_dek"`
}

// VaultEnvelope represents the top-level env.lock JSON schema.
type VaultEnvelope struct {
	Version    int              `json:"version"`
	Cipher     string           `json:"cipher"`
	IV         string           `json:"iv"`
	Ciphertext string           `json:"ciphertext"`
	Recipients []RecipientEntry `json:"recipients"`
}

// PubKeys is untrusted: anyone can write a vault that decrypts for any list of keys.
func (v *VaultEnvelope) PubKeys() []string {
	keys := make([]string, len(v.Recipients))
	for i, r := range v.Recipients {
		keys[i] = r.PubKey
	}
	return keys
}

// PayloadAAD binds the recipient list to the payload, so editing the list breaks decryption.
func (v *VaultEnvelope) PayloadAAD() []byte {
	aad, _ := json.Marshal(v.PubKeys())
	return aad
}
