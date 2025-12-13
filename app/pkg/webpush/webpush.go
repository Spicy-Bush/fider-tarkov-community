package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
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
	"sync"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/env"
	"github.com/Spicy-Bush/fider-tarkov-community/app/pkg/errors"
	"golang.org/x/crypto/hkdf"
)

var (
	vapidKeys     *VAPIDKeys
	vapidKeysOnce sync.Once
	vapidKeysErr  error
	httpClient    = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 4096))
		},
	}
)

type VAPIDKeys struct {
	PublicKey  []byte
	PrivateKey *ecdsa.PrivateKey
	publicB64  string
}

type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type Notification struct {
	Title   string `json:"title"`
	Body    string `json:"body,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Badge   string `json:"badge,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Data    any    `json:"data,omitempty"`
	URL     string `json:"url,omitempty"`
	Actions []struct {
		Action string `json:"action"`
		Title  string `json:"title"`
	} `json:"actions,omitempty"`
}

func ecdsaPrivateKeyFromECDH(privKey *ecdh.PrivateKey, pubKey *ecdh.PublicKey) (*ecdsa.PrivateKey, error) {
	pubBytes := pubKey.Bytes()
	if len(pubBytes) != 65 || pubBytes[0] != 0x04 {
		return nil, errors.New("invalid uncompressed public key format")
	}

	x := new(big.Int).SetBytes(pubBytes[1:33])
	y := new(big.Int).SetBytes(pubBytes[33:65])

	return &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     x,
			Y:     y,
		},
		D: new(big.Int).SetBytes(privKey.Bytes()),
	}, nil
}

func GetVAPIDKeys() (*VAPIDKeys, error) {
	vapidKeysOnce.Do(func() {
		if !env.IsWebPushEnabled() {
			vapidKeysErr = errors.New("web push is not configured")
			return
		}

		pubKeyBytes, err := base64.RawURLEncoding.DecodeString(env.Config.WebPush.VAPIDPublicKey)
		if err != nil {
			vapidKeysErr = errors.Wrap(err, "failed to decode VAPID public key")
			return
		}

		privKeyBytes, err := base64.RawURLEncoding.DecodeString(env.Config.WebPush.VAPIDPrivateKey)
		if err != nil {
			vapidKeysErr = errors.Wrap(err, "failed to decode VAPID private key")
			return
		}

		ecdhPubKey, err := ecdh.P256().NewPublicKey(pubKeyBytes)
		if err != nil {
			vapidKeysErr = errors.Wrap(err, "invalid VAPID public key")
			return
		}

		ecdhPrivKey, err := ecdh.P256().NewPrivateKey(privKeyBytes)
		if err != nil {
			vapidKeysErr = errors.Wrap(err, "invalid VAPID private key")
			return
		}

		ecdsaPrivKey, err := ecdsaPrivateKeyFromECDH(ecdhPrivKey, ecdhPubKey)
		if err != nil {
			vapidKeysErr = errors.Wrap(err, "failed to convert ECDH key to ECDSA")
			return
		}

		vapidKeys = &VAPIDKeys{
			PublicKey:  pubKeyBytes,
			PrivateKey: ecdsaPrivKey,
			publicB64:  env.Config.WebPush.VAPIDPublicKey,
		}
	})

	return vapidKeys, vapidKeysErr
}

func GetPublicKeyBase64() string {
	keys, err := GetVAPIDKeys()
	if err != nil {
		return ""
	}
	return keys.publicB64
}

func SendNotification(ctx context.Context, sub *Subscription, notification *Notification, ttl int) error {
	keys, err := GetVAPIDKeys()
	if err != nil {
		return err
	}

	payload, err := json.Marshal(notification)
	if err != nil {
		return errors.Wrap(err, "failed to marshal notification")
	}

	encrypted, err := encryptPayload(sub, payload)
	if err != nil {
		return errors.Wrap(err, "failed to encrypt payload")
	}

	vapidHeader, err := generateVAPIDHeader(sub.Endpoint, keys)
	if err != nil {
		return errors.Wrap(err, "failed to generate VAPID header")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(encrypted.ciphertext))
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Authorization", vapidHeader)
	req.Header.Set("TTL", fmt.Sprintf("%d", ttl))
	req.Header.Set("Urgency", "normal")

	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to send push notification")
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return &SubscriptionExpiredError{Endpoint: sub.Endpoint}
	}

	return errors.New("push notification failed with status %d: %s", resp.StatusCode, string(body))
}

type SubscriptionExpiredError struct {
	Endpoint string
}

func (e *SubscriptionExpiredError) Error() string {
	return fmt.Sprintf("subscription expired: %s", e.Endpoint)
}

func IsSubscriptionExpired(err error) bool {
	_, ok := err.(*SubscriptionExpiredError)
	return ok
}

type encryptedPayload struct {
	ciphertext []byte
}

func encryptPayload(sub *Subscription, payload []byte) (*encryptedPayload, error) {
	clientPubKeyBytes, err := base64.RawURLEncoding.DecodeString(sub.Keys.P256dh)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode p256dh key")
	}

	authSecret, err := base64.RawURLEncoding.DecodeString(sub.Keys.Auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode auth secret")
	}

	clientPubKey, err := ecdh.P256().NewPublicKey(clientPubKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse client public key")
	}

	serverPrivKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate server key")
	}
	serverPubKey := serverPrivKey.PublicKey()

	sharedSecret, err := serverPrivKey.ECDH(clientPubKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compute shared secret")
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, errors.Wrap(err, "failed to generate salt")
	}

	ikm := deriveIKM(authSecret, sharedSecret, clientPubKeyBytes, serverPubKey.Bytes())

	prk := hkdfExtract(salt, ikm)
	contentKey := hkdfExpand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(prk, []byte("Content-Encoding: nonce\x00"), 12)

	block, err := aes.NewCipher(contentKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create cipher")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create GCM")
	}

	paddedPayload := append(payload, 0x02)
	ciphertext := gcm.Seal(nil, nonce, paddedPayload, nil)

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	buf.Write(salt)
	binary.Write(buf, binary.BigEndian, uint32(4096))
	buf.WriteByte(byte(len(serverPubKey.Bytes())))
	buf.Write(serverPubKey.Bytes())
	buf.Write(ciphertext)

	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())

	return &encryptedPayload{ciphertext: result}, nil
}

func deriveIKM(authSecret, sharedSecret, clientPubKey, serverPubKey []byte) []byte {
	authInfo := make([]byte, 0, len("WebPush: info\x00")+len(clientPubKey)+len(serverPubKey))
	authInfo = append(authInfo, []byte("WebPush: info\x00")...)
	authInfo = append(authInfo, clientPubKey...)
	authInfo = append(authInfo, serverPubKey...)

	prk := hkdfExtract(authSecret, sharedSecret)
	return hkdfExpand(prk, authInfo, 32)
}

func hkdfExtract(salt, ikm []byte) []byte {
	return hkdf.Extract(sha256.New, ikm, salt)
}

func hkdfExpand(prk, info []byte, length int) []byte {
	reader := hkdf.Expand(sha256.New, prk, info)
	result := make([]byte, length)
	reader.Read(result)
	return result
}

func generateVAPIDHeader(endpoint string, keys *VAPIDKeys) (string, error) {
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse endpoint URL")
	}

	aud := fmt.Sprintf("%s://%s", endpointURL.Scheme, endpointURL.Host)
	now := time.Now().Unix()
	exp := now + 12*60*60

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))

	claims := fmt.Sprintf(`{"aud":"%s","exp":%d,"sub":"mailto:%s"}`, aud, exp, env.Config.Email.NoReply)
	claimsB64 := base64.RawURLEncoding.EncodeToString([]byte(claims))

	unsigned := header + "." + claimsB64

	hash := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, keys.PrivateKey, hash[:])
	if err != nil {
		return "", errors.Wrap(err, "failed to sign VAPID token")
	}

	sigBytes := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)

	signature := base64.RawURLEncoding.EncodeToString(sigBytes)
	jwt := unsigned + "." + signature

	return fmt.Sprintf("vapid t=%s, k=%s", jwt, keys.publicB64), nil
}

func GenerateVAPIDKeys() (publicKey, privateKey string, err error) {
	privKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to generate ECDH key")
	}

	publicKey = base64.RawURLEncoding.EncodeToString(privKey.PublicKey().Bytes())
	privateKey = base64.RawURLEncoding.EncodeToString(privKey.Bytes())

	return publicKey, privateKey, nil
}
