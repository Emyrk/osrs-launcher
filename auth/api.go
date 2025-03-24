package auth

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

//go:embed auth.html
var callbackFE string

type AccountDisplayName struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Suffix      string `json:"suffix"`
}

// DisplayName works!
func (a *JagexAccountAuth) DisplayName(ctx context.Context, cfg *oauth2.Config, sub string) (AccountDisplayName, error) {
	cli := cfg.Client(ctx, &a.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.jagex.com/v1/users/%s/displayName", sub), nil)
	if err != nil {
		return AccountDisplayName{}, fmt.Errorf("fetch accounts req: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return AccountDisplayName{}, fmt.Errorf("fetch display name: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return AccountDisplayName{}, fmt.Errorf("fetch display name: %w", fmt.Errorf("status code: %d", resp.StatusCode))
	}

	var displayName AccountDisplayName
	return displayName, json.NewDecoder(resp.Body).Decode(&displayName)
}

type UserInfo struct {
	Amr      []string `json:"amr"`
	Aud      []string `json:"aud"`
	AuthTime int      `json:"auth_time"`
	Iat      int      `json:"iat"`
	Iss      string   `json:"iss"`
	Nickname string   `json:"nickname"`
	Rat      int      `json:"rat"`
	Sub      string   `json:"sub"`
}

// UserInfo works!
func (a *JagexAccountAuth) UserInfo(ctx context.Context, cfg *oauth2.Config) (UserInfo, error) {
	cli := cfg.Client(ctx, &a.Token)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(sessionsPayload{
		IDToken: a.IDToken,
	})
	if err != nil {
		return UserInfo{}, fmt.Errorf("encoding idtoken payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://account.jagex.com/userinfo", nil)
	if err != nil {
		return UserInfo{}, fmt.Errorf("fetch accounts req: %w", err)
	}
	req.Header.Add("Accept", "application/json")

	resp, err := cli.Do(req)
	if err != nil {
		return UserInfo{}, fmt.Errorf("fetch accounts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UserInfo{}, fmt.Errorf("fetch user info: %w", fmt.Errorf("status code: %d", resp.StatusCode))
	}

	var info UserInfo
	return info, json.NewDecoder(resp.Body).Decode(&info)
}

func (a *JagexAccountAuth) AuthConsent(ctx context.Context, cfg *oauth2.Config) (string, <-chan struct{}, error) {
	authURL, err := url.Parse(cfg.Endpoint.AuthURL)
	if err != nil {
		return "", nil, fmt.Errorf("parsing auth url: %w", err)
	}

	state := randomState()
	nonce := uuid.NewString()
	vals := url.Values{
		"client_id":     {"1fddee4e-b100-4f4e-b2b0-097f9088f9d2"},
		"response_type": {"id_token code"},
		"scope":         {"openid offline"},
		"prompt":        {"consent"},
		"state":         {state},
		"id_token_hint": {a.IDToken},
		"nonce":         {nonce},
		"redirect_uri":  {"http://localhost"},
	}
	authURL.RawQuery = vals.Encode()

	//secondCfg := *cfg
	//cfg.ClientID = "1fddee4e-b100-4f4e-b2b0-097f9088f9d2"

	srvCtx, cancel := context.WithCancelCause(ctx)
	srv := http.Server{
		Addr: "0.0.0.0:80",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info().
				Str("url", r.URL.String()).
				Msg("localhost callback")
			if r.URL.RawQuery == "" {
				_, _ = w.Write([]byte(callbackFE))
				return
			}

			// OIDC errors can be returned as query parameters. This can happen
			// if for example we are providing and invalid scope.
			// We should terminate the OIDC process if we encounter an error.
			errorMsg := r.URL.Query().Get("error")
			errorDescription := r.URL.Query().Get("error_description")
			errorURI := r.URL.Query().Get("error_uri")
			if errorMsg != "" {
				// Combine the errors into a single string if either is provided.
				if errorDescription == "" && errorURI != "" {
					errorDescription = fmt.Sprintf("error_uri: %s", errorURI)
				} else if errorDescription != "" && errorURI != "" {
					errorDescription = fmt.Sprintf("%s, error_uri: %s", errorDescription, errorURI)
				}
				errorMsg = fmt.Sprintf("Encountered error in oidc process: %s", errorMsg)
				err = fmt.Errorf(errorMsg)
				log.Err(err).Msg("Error from oauth exchange")
				cancel(err)
				_, _ = w.Write([]byte(errorMsg))
				return
			}

			code := strings.TrimSpace(r.URL.Query().Get("code"))
			var _ = code
			newIDToken := strings.TrimSpace(r.URL.Query().Get("id_token"))

			a.GameIDToken = newIDToken
			cancel(nil)
		}),
		BaseContext: func(_ net.Listener) context.Context {
			return srvCtx
		},
	}
	go func() {
		_ = srv.ListenAndServe()
		log.Info().Msg("http server closed")
	}()
	go func() {
		select {
		case <-srvCtx.Done():
			_ = srv.Close()
		}
	}()

	return authURL.String(), srvCtx.Done(), nil
}

type jagexError struct {
	TraceID string `json:"traceId"`
	SpanID  string `json:"spanId"`
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
	ID      string `json:"id"`
}

type JagexCharacter struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	UserHash    string `json:"userHash"`
}

func (a *JagexAccountAuth) Accounts(ctx context.Context) error {
	if a.Session == "" {
		return fmt.Errorf("empty session, cannot fetch accounts")
	}

	cli := http.DefaultClient
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://auth.runescape.com/game-session/v1/accounts", nil)
	if err != nil {
		return fmt.Errorf("fetch accounts req: %w", err)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+a.Session)

	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("fetch characters: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		a.Session = ""
		return fmt.Errorf("session token invalid, deleting, and you must reauthenticate")
	}

	if resp.StatusCode != http.StatusOK {
		a.Session = ""
		var jError jagexError
		json.NewDecoder(resp.Body).Decode(&jError)
		return fmt.Errorf("fetching characters failed (status %d), going to delete the session :: %s", resp.StatusCode, jError.Message)
	}

	var accts []JagexCharacter
	err = json.NewDecoder(resp.Body).Decode(&accts)
	if err != nil {
		return fmt.Errorf("decoding characters: %w", err)
	}

	if len(accts) == 0 {
		return fmt.Errorf("empty character list, go make one")
	}
	a.Characters = accts

	return nil
}

type sessionsPayload struct {
	IDToken string `json:"idToken"`
}

type sessionResponse struct {
	SessionID string `json:"sessionId"`
}

func (a *JagexAccountAuth) Sessions(ctx context.Context, cfg *oauth2.Config) error {
	cli := cfg.Client(ctx, &a.Token)

	cli = &http.Client{}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(sessionsPayload{
		IDToken: a.GameIDToken,
	})
	if err != nil {
		return fmt.Errorf("encoding idtoken payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://auth.jagex.com/game-session/v1/sessions", &buf)
	if err != nil {
		return fmt.Errorf("fetch rsn request: %w", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("fetch rsn: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var jError jagexError
		json.NewDecoder(resp.Body).Decode(&jError)
		if jError.Code == "ID_TOKEN_ALREADY_USED" {
			log.Err(fmt.Errorf("id token already used")).Msg("deleting used id token")
			a.GameIDToken = ""
		}

		return fmt.Errorf("fetch session: %w", fmt.Errorf("status code: %d", resp.StatusCode))
	}

	// Always delete the now used id token
	a.GameIDToken = ""

	var session sessionResponse
	err = json.NewDecoder(resp.Body).Decode(&session)
	if err != nil {
		return fmt.Errorf("decoding session: %w", err)
	}

	if session.SessionID == "" {
		return fmt.Errorf("empty session id")
	}

	a.Session = session.SessionID
	return nil
}
