package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/coreos/go-oidc"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

type JagexAccountAuth struct {
	Token       oauth2.Token     `json:"token"`
	IDToken     string           `json:"id_token"`
	GameIDToken string           `json:"game_id_token"`
	Session     string           `json:"session"`
	Characters  []JagexCharacter `json:"characters"`
}

func (a *JagexAccountAuth) Refresh(ctx context.Context, cfg *oauth2.Config) error {
	before := a.Token.AccessToken
	token, err := cfg.TokenSource(ctx, &a.Token).Token()
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}

	after := token.AccessToken
	idToken := a.IDToken
	if before != after {
		idToken = token.Extra("id_token").(string)
		log.Info().Msg("token refreshed")
	}

	a.Token = *token
	a.IDToken = idToken
	return nil
}

func (a JagexAccountAuth) VerifyAll(ctx context.Context, verifier *oidc.IDTokenVerifier) (*oidc.IDToken, error) {
	if !a.Token.Expiry.IsZero() && time.Now().After(a.Token.Expiry) {
		return nil, fmt.Errorf("token expired")
	}

	idToken, err := a.VerifyIDToken(ctx, verifier)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}

	return idToken, nil
}

func (a JagexAccountAuth) VerifyIDToken(ctx context.Context, verifier *oidc.IDTokenVerifier) (*oidc.IDToken, error) {
	parsed, err := verifier.Verify(ctx, a.IDToken)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}

	if strings.Contains(a.IDToken, "com_jagex_auth_desktop_launcher") {
		err = parsed.VerifyAccessToken(a.Token.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("invalid access token: %w", err)
		}
	}

	return parsed, nil
}

// https://account.jagex.com/.well-known/openid-configuration
func JagexProvider() (*oidc.Provider, error) {
	return oidc.NewProvider(context.Background(), "https://account.jagex.com/")
}

// JagexVerifier is missing the jagex keyset, Idk where to get it.
func JagexVerifier(provider *oidc.Provider) *oidc.IDTokenVerifier {
	return provider.Verifier(&oidc.Config{
		ClientID:          "com_jagex_auth_desktop_launcher",
		SkipClientIDCheck: true,
	})
}

func JagexOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     "com_jagex_auth_desktop_launcher",
		ClientSecret: "",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://account.jagex.com/oauth2/auth",
			TokenURL: "https://account.jagex.com/oauth2/token",
		},
		RedirectURL: "https://secure.runescape.com/m=weblogin/launcher-redirect",
		Scopes: []string{
			"openid", "offline", "gamesso.token.create", "user.profile.read",
		},
	}
}

// Look at 	"static navigateToAuthConsent(origin: string, id_token: string, nonce: string) {"
func AuthenticateJagexAccount(ctx context.Context, cfg *oauth2.Config) (*JagexAccountAuth, error) {
	verifier := oauth2.GenerateVerifier()

	// https://github.com/Adamcake/Bolt/blob/master/app/src/lib/Services/AuthService.ts#L34-L46
	u := cfg.AuthCodeURL(randomState(),
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),

		// Taken from pcap
		oauth2.SetAuthURLParam("prompt", "login"),
		oauth2.SetAuthURLParam("flow", "launcher"),
	)

	fmt.Println("Visit this url")
	fmt.Println(u)

	var jagexCode string
	err := huh.NewInput().
		Title("Input the url returned").
		Value(&jagexCode).
		Run()
	if err != nil {
		return nil, fmt.Errorf("input: %w", err)
	}

	// jagex:code=8s9YzvGxdFVrZCV6o4-d5mvLzv0cU1vImzGvquOFBJU.x-5b1MEm3p5hjDxAJ4XMszE0uKg5nWYGMu_qrYcfZqc,state=12354124124,intent=social_auth
	if !strings.HasPrefix(jagexCode, "jagex:") {
		fmt.Println("Invalid code")
		return nil, fmt.Errorf("invalid code, must start with 'jagex:'")
	}

	jagexCode = strings.TrimPrefix(jagexCode, "jagex:")
	var code, state, intent string
	list := strings.Split(jagexCode, ",")
	for _, item := range list {
		parts := strings.Split(item, "=")
		switch parts[0] {
		case "code":
			code = parts[1]
		case "state":
			state = parts[1]
		case "intent":
			intent = parts[1]
		}
	}
	var _, _ = state, intent
	//fmt.Println(state, intent)

	token, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange: %w", err)
	}
	idToken := token.Extra("id_token").(string)

	return &JagexAccountAuth{
		Token:   *token,
		IDToken: idToken,
	}, nil
}

func randomState() string {
	x := make([]byte, 16)
	_, _ = rand.Read(x)
	return hex.EncodeToString(x)
}
