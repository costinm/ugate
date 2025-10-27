package oidc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/go-jose/go-jose/v4"

	"github.com/costinm/meshauth"
)

// github.com/coreos/go-oidc library wraps square/go-jose.v2 - it has code to download the JWKS
// from the OIDC well known location.
//
// It has some google-specific code, but mainly uses keySet.VerifySignature.

type AuthOIDC struct {
	jwtProviders map[string]*oidc.IDTokenVerifier
	Audience     string
}

func ConvertJWKS(i *meshauth.TrustConfig) error {
	body := []byte(i.Jwks)

	var keySet jose.JSONWebKeySet
	err := json.Unmarshal(body, &keySet)
	if err != nil {
		return err
	}

	// Map of 'kid' to key
	i.KeysByKid = map[string]interface{}{}
	for _, ks := range keySet.Keys {
		i.KeysByKid[ks.KeyID] = ks.Key
	}

	return nil
}

func (a *AuthOIDC) Verify(ctx context.Context, v *meshauth.TrustConfig, raw string) error {
	if v.Key == nil {
		err := a.Init(ctx, v)
		if err != nil {
			return err
		}
	}

	if ver, ok := v.Key.(*oidc.IDTokenVerifier); ok {
		_, err := ver.Verify(ctx, raw)
		return err
	}

	return nil
}

func (a *AuthOIDC) Init(ctx context.Context, v *meshauth.TrustConfig) error {
	if a.jwtProviders == nil {
		a.jwtProviders = map[string]*oidc.IDTokenVerifier{}
	}
	var ver *oidc.IDTokenVerifier
	if len(v.JwksUri) > 0 {
		// oidc.KeySet embed and refresh cached jose.JSONWebKey
		// It is unfortunately just an interface - doesn't expose the actual keys.
		keySet := oidc.NewRemoteKeySet(context.Background(), v.JwksUri)
		ver = oidc.NewVerifier(v.Issuer, keySet, &oidc.Config{SkipClientIDCheck: true})
	} else {
		// Use Issuer directly to download the OIDC document and extract the JwksUri
		// Will create the oidc.KeySet internally. No caching for the OIDC
		provider, err := oidc.NewProvider(context.Background(), v.Issuer)
		if err != nil {
			// OIDC discovery may fail, e.g. http request for the OIDC server may fail.
			// Instead of a permanent failre, this will be done on-demand, so other providers
			// may continue to work.
			slog.Info("Issuer not found, skipping", "iss", v, "error", err)
			return err
		}
		ver = provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
	}
	v.Key = ver
	a.jwtProviders[v.Issuer] = ver
	return nil
}

func (a *AuthOIDC) InitJWT(issuers []string) {
	a.jwtProviders = map[string]*oidc.IDTokenVerifier{}
	if len(issuers) == 0 {
		return
	}
	for _, i := range issuers {
		provider, err := oidc.NewProvider(context.Background(), i)
		if err != nil {
			slog.Info("Issuer not found, skipping", "iss", i,
				"error", err)
			continue
		}
		verifier := provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
		a.jwtProviders[i] = verifier
	}
}

// From go-oidc/verify.go
func parseJWT(p string) ([]byte, error) {
	parts := strings.Split(p, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("oidc: malformed jwt, expected 3 parts got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oidc: malformed jwt payload: %v", err)
	}
	return payload, nil
}

func (a *AuthOIDC) CheckJwt(password string) (tok map[string]string, e error) {
	// Validate a JWT as password.
	// Alternative: JWKS_URL and NewRemoteKeySet
	// TODO: init at startup, reuse
	jwt := meshauth.DecodeJWT(password)
	if jwt == nil {
		return nil, errors.New("invalid JWT")
	}

	verifier := a.jwtProviders[jwt.Iss]
	if verifier == nil {
		return nil, errors.New("unknown issuer ")
	}

	idt, err := verifier.Verify(context.Background(), string(password))
	if err == nil {
		// claims - tricky to extract
		slog.Info("AuthJwt", "iss", idt.Issuer,
			"aud", idt.Audience, "sub", idt.Subject,
			"err", err)
		if len(idt.Audience) == 0 || !strings.HasPrefix(idt.Audience[0], a.Audience) {
			return nil, errors.New("Invalid audience")
		}
		// TODO: check audience against config, domain
		return map[string]string{"sub": idt.Subject}, nil
	} else {
		slog.Info("JWT failed", "error", err, "pass", password)
		e = err
	}
	//}
	return
}
