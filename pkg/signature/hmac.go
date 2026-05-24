package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const signaturePrefix = "sha256="

func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

func Verify(secret string, body []byte, signature string) bool {
	expected := Sign(secret, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func Header() string {
	return "X-Notiq-Signature"
}

func ExampleVerification() string {
	return fmt.Sprintf(`
	receivedSig := r.Header.Get("%s")
	body, _ := io.ReadAll(r.Body)
	if !signature.Verify(yourSecret, body, receivedSig) {
	    http.Error(w, "invalid signature", 401)
	    return
	}`, Header())
}
