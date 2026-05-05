package token

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	eddsaKeyStoreAllKids = "auth:eddsa:keys:all"
	eddsaKeyStoreCurrent = "auth:eddsa:keys:current_kid"
	eddsaKeyStorePrivate = "auth:eddsa:keys:%s:private"
	eddsaKeyStorePublic  = "auth:eddsa:keys:%s:public"
	eddsaKeyStoreMeta    = "auth:eddsa:keys:%s:meta"
)

type eddsaKeyMeta struct {
	Status    KeyStatus `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	RotatedAt time.Time `json:"rotated_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type eddsaRedisKeyStore struct {
	client *redis.Client
}

func newEDDSARedisKeyStore(client *redis.Client) *eddsaRedisKeyStore {
	return &eddsaRedisKeyStore{client: client}
}

func (s *eddsaRedisKeyStore) loadAllKids(ctx context.Context) ([]string, error) {
	kids, err := s.client.SMembers(ctx, eddsaKeyStoreAllKids).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("smembers all kids: %w", err)
	}
	return kids, nil
}

func (s *eddsaRedisKeyStore) loadCurrentKID(ctx context.Context) (string, error) {
	kid, err := s.client.Get(ctx, eddsaKeyStoreCurrent).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("get current kid: %w", err)
	}
	return kid, nil
}

func (s *eddsaRedisKeyStore) loadKeyMeta(ctx context.Context, kid string) (*eddsaKeyMeta, error) {
	key := fmt.Sprintf(eddsaKeyStoreMeta, kid)
	data, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall key meta: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var meta eddsaKeyMeta
	meta.Status = KeyStatus(data["status"])

	if ca, _ := strconv.ParseInt(data["created_at"], 10, 64); ca > 0 {
		meta.CreatedAt = time.Unix(ca, 0)
	}
	if ra, _ := strconv.ParseInt(data["rotated_at"], 10, 64); ra > 0 {
		meta.RotatedAt = time.Unix(ra, 0)
	}
	if ea, _ := strconv.ParseInt(data["expires_at"], 10, 64); ea > 0 {
		meta.ExpiresAt = time.Unix(ea, 0)
	}
	return &meta, nil
}

func (s *eddsaRedisKeyStore) loadPrivateKey(ctx context.Context, kid string) (ed25519.PrivateKey, error) {
	data, err := s.client.Get(ctx, fmt.Sprintf(eddsaKeyStorePrivate, kid)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get private key: %w", err)
	}
	return parseEd25519PrivateKeyFromPEM(data)
}

func (s *eddsaRedisKeyStore) loadPublicKey(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	data, err := s.client.Get(ctx, fmt.Sprintf(eddsaKeyStorePublic, kid)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("get public key: %w", err)
	}
	return parseEd25519PublicKeyFromPEM(data)
}

func (s *eddsaRedisKeyStore) storeKey(ctx context.Context, kid string, privateKey ed25519.PrivateKey, ttl time.Duration) error {
	now := time.Now().UTC()
	privPEM, err := encodeEd25519PrivateKeyToPEM(privateKey)
	if err != nil {
		return err
	}
	pubPEM, err := encodeEd25519PublicKeyToPEM(privateKey.Public().(ed25519.PublicKey))
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.SAdd(ctx, eddsaKeyStoreAllKids, kid)
	pipe.Set(ctx, eddsaKeyStoreCurrent, kid, 0)
	pipe.Set(ctx, fmt.Sprintf(eddsaKeyStorePrivate, kid), privPEM, ttl)
	pipe.Set(ctx, fmt.Sprintf(eddsaKeyStorePublic, kid), pubPEM, ttl)

	metaKey := fmt.Sprintf(eddsaKeyStoreMeta, kid)
	pipe.HSet(ctx, metaKey, map[string]interface{}{
		"status":     string(KeyStatusActive),
		"created_at": now.Unix(),
		"rotated_at": 0,
		"expires_at": now.Add(ttl).Unix(),
	})
	pipe.Expire(ctx, metaKey, ttl)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("store key pipeline: %w", err)
	}
	return nil
}

func (s *eddsaRedisKeyStore) retireKey(ctx context.Context, kid string) error {
	meta, err := s.loadKeyMeta(ctx, kid)
	if err != nil {
		return err
	}
	if meta == nil {
		return nil
	}

	now := time.Now().UTC()
	metaKey := fmt.Sprintf(eddsaKeyStoreMeta, kid)

	pipe := s.client.Pipeline()
	pipe.HSet(ctx, metaKey, map[string]interface{}{
		"status":     string(KeyStatusRetired),
		"rotated_at": now.Unix(),
	})
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update retired key meta: %w", err)
	}
	return nil
}

func encodeEd25519PrivateKeyToPEM(key ed25519.PrivateKey) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("marshal pkcs8 private key: %w", err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	return string(pem.EncodeToMemory(block)), nil
}

func encodeEd25519PublicKeyToPEM(key ed25519.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", fmt.Errorf("marshal pkix public key: %w", err)
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	return string(pem.EncodeToMemory(block)), nil
}

func parseEd25519PrivateKeyFromPEM(pemData string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemData)))
	if block == nil {
		return nil, fmt.Errorf("invalid pem data")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8 private key: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ed25519 private key")
	}
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key size")
	}
	return priv, nil
}

func parseEd25519PublicKeyFromPEM(pemData string) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemData)))
	if block == nil {
		return nil, fmt.Errorf("invalid pem data")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkix public key: %w", err)
	}
	edPub, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ed25519 public key")
	}
	if len(edPub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 public key size")
	}
	return edPub, nil
}
