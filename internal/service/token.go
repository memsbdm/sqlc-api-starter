package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/memsbdm/restaurant-api/config"
	"github.com/memsbdm/restaurant-api/internal/cache"
	"github.com/memsbdm/restaurant-api/pkg/keys"
	"github.com/memsbdm/restaurant-api/pkg/security"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type TokenService interface {
	GenerateOAT(ctx context.Context, keyPrefix keys.OAT, data string, ttl time.Duration) (string, error)
	VerifyOAT(ctx context.Context, keyPrefix keys.OAT, encodedOAT string) (string, error)
	GenerateSPT(ctx context.Context, keyPrefix keys.SPT, data string, ttl time.Duration) (string, error)
	VerifySPT(ctx context.Context, keyPrefix keys.SPT, encodedSPT string) (string, error)
	RevokeSPT(ctx context.Context, keyPrefix keys.SPT, encodedSPT string) error
}

type tokenService struct {
	cfg   *config.Security
	cache cache.Cache
}

func NewTokenService(cfg *config.Security, cache cache.Cache) *tokenService {
	return &tokenService{
		cfg:   cfg,
		cache: cache,
	}
}

func (s *tokenService) GenerateOAT(ctx context.Context, keyPrefix keys.OAT, data string, ttl time.Duration) (string, error) {
	oat, err := security.GenerateRandomString(32)
	if err != nil {
		return "", err
	}

	err = s.cache.Set(ctx, cache.GenerateKey(string(keyPrefix), oat), []byte(data), ttl)
	if err != nil {
		return "", err
	}

	signature := security.SignString(oat, s.cfg.OATSecret)
	signedOAT := fmt.Sprintf("%s.%s", oat, signature)

	return security.EncodeTokenURLSafe(signedOAT), nil
}

func (s *tokenService) VerifyOAT(ctx context.Context, keyPrefix keys.OAT, encodedOAT string) (string, error) {
	decodedOAT, err := security.DecodeTokenURLSafe(encodedOAT)
	if err != nil {
		return "", ErrInvalidToken
	}

	parts := strings.Split(decodedOAT, ".")
	if len(parts) != 2 {
		return "", ErrInvalidToken
	}

	oat, signature := parts[0], parts[1]
	hasValidSignature := security.VerifySignature(oat, signature, s.cfg.OATSecret)
	if !hasValidSignature {
		return "", ErrInvalidToken
	}

	data, err := s.cache.Get(ctx, cache.GenerateKey(string(keyPrefix), oat))
	if err != nil {
		if errors.Is(err, cache.ErrCacheNotFound) {
			return "", ErrInvalidToken
		}
		return "", err
	}

	return string(data), nil
}

func (s *tokenService) GenerateSPT(ctx context.Context, keyPrefix keys.SPT, data string, ttl time.Duration) (string, error) {
	// Format: <user_id>.<signature>
	signature := security.SignString(data, s.cfg.SPTSecret)
	signedSPT := fmt.Sprintf("%s.%s", data, signature)

	err := s.cache.Set(ctx, cache.GenerateKey(string(keyPrefix), data), []byte(data), ttl)
	if err != nil {
		return "", err
	}

	return security.EncodeTokenURLSafe(signedSPT), nil
}

func (s *tokenService) VerifySPT(ctx context.Context, keyPrefix keys.SPT, encodedSPT string) (string, error) {
	decodedSPT, err := security.DecodeTokenURLSafe(encodedSPT)
	if err != nil {
		return "", ErrInvalidToken
	}

	parts := strings.Split(decodedSPT, ".")
	if len(parts) != 2 {
		return "", ErrInvalidToken
	}

	spt, signature := parts[0], parts[1]
	hasValidSignature := security.VerifySignature(spt, signature, s.cfg.SPTSecret)
	if !hasValidSignature {
		return "", ErrInvalidToken
	}

	data, err := s.cache.Get(ctx, cache.GenerateKey(string(keyPrefix), spt))
	if err != nil {
		if errors.Is(err, cache.ErrCacheNotFound) {
			return "", ErrInvalidToken
		}
		return "", err
	}

	return string(data), nil
}

func (s *tokenService) RevokeSPT(ctx context.Context, keyPrefix keys.SPT, data string) error {
	return s.cache.Delete(ctx, cache.GenerateKey(string(keyPrefix), data))
}
