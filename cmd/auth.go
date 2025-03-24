package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/Emyrk/osrs-launcher/auth"
	"github.com/Emyrk/osrs-launcher/config"
	"github.com/charmbracelet/huh"
	"github.com/rs/zerolog/log"

	"github.com/coder/serpent"
)

func (r *Root) Auth() *serpent.Command {
	var outputDestination string
	return &serpent.Command{
		Use: "auth",
		Options: serpent.OptionSet{
			{
				Name:          "Output Destination",
				Description:   "Place to output the credentials.properties file to.",
				Flag:          "output-destination",
				FlagShorthand: "O",
				Default:       "$HOME/.runelite/credentials.properties",
				Value:         serpent.StringOf(&outputDestination),
			},
		},
		Middleware: serpent.Chain(r.LoggerMW(), UseProxy),
		Handler: func(i *serpent.Invocation) error {
			ctx := i.Context()
			err := auth.TestPort80()
			if err != nil {
				if strings.Contains(err.Error(), "permission denied") {
					return fmt.Errorf("port 80 is blocked, you must grant permission for this program to listen on this port. Run 'sudo setcap CAP_NET_BIND_SERVICE=+eip `which %s`'", os.Args[0])
				}
				return fmt.Errorf("testing port 80: %w", err)
			}

			provider, err := auth.JagexProvider()
			if err != nil {
				return fmt.Errorf("getting provider: %w", err)
			}
			verifier := auth.JagexVerifier(provider)

			root := config.DefaultDir().Init()
			all, err := root.Accounts()
			options := make([]huh.Option[string], 0, len(all))
			options = append(options, huh.NewOption("New Account", "new"))
			for _, account := range all {
				options = append(options, huh.Option[string]{
					Key:   account.Name(),
					Value: account.Name(),
				})
			}

			var sel string
			err = huh.NewSelect[string]().
				Title("Select Jagex account to authenticate").
				Options(options...).
				Value(&sel).
				Run()
			if err != nil {
				return fmt.Errorf("selecting: %w", err)
			}

			var acct *auth.JagexAccountAuth
			cfg := auth.JagexOAuthConfig()

			if sel == "new" {
				newToken, err := auth.AuthenticateJagexAccount(ctx, cfg)
				if err != nil {
					return fmt.Errorf("getting oauth token: %w", err)
				}
				acct = newToken
			} else {
				account := root.Account(sel)
				existingToken, err := account.Token()
				if err != nil {
					return fmt.Errorf("getting token from save: %w", err)
				}
				acct = &existingToken
			}

			log.Info().
				Msg("Refreshing token if needed")
			err = acct.Refresh(ctx, cfg)
			if err != nil {
				return fmt.Errorf("refresh token: %w", err)
			}

			idToken, err := acct.VerifyAll(ctx, verifier)
			log.Info().Err(err).Msg("Verifying token")
			if err != nil {
				return fmt.Errorf("verifying token: %w", err)
			}
			var _ = idToken

			userInfo, err := acct.UserInfo(ctx, cfg)
			if err != nil {
				return fmt.Errorf("getting user info: %w", err)
			}

			displayName, err := acct.DisplayName(ctx, cfg, userInfo.Sub)
			if err != nil {
				return fmt.Errorf("getting display name: %w", err)
			}

			//if slices.Contains(idToken.Audience, "com_jagex_auth_desktop_launcher") {
			if acct.GameIDToken == "" && acct.Session == "" {
				// We need to upgrade the consent
				consent, done, err := acct.AuthConsent(ctx, cfg)
				if err != nil {
					return fmt.Errorf("getting auth consent: %w", err)
				}
				fmt.Printf("Consent URL, please visit: %s\n", consent)

				select {
				case <-done:
					log.Info().Msg("Consent complete")
				case <-ctx.Done():
					log.Info().Msg("Context cancelled")
				}
			}

			log.Info().
				Str("display_name", displayName.DisplayName).
				Msg("Saving token to disk")
			defer func() {
				err = root.Account(displayName.DisplayName).SaveToken(acct)
				if err != nil {
					log.Error().
						Err(err).
						Msg("saving token to disk")
				}
			}()

			if acct.Session == "" {
				err = acct.Sessions(ctx, cfg)
				if err != nil {
					return fmt.Errorf("getting sessions: %w", err)
				}
			}

			// Make sure the session is still valid
			err = acct.Accounts(ctx)
			if err != nil {
				return fmt.Errorf("getting sessions: %w", err)
			}

			opts := make([]huh.Option[string], 0, len(acct.Characters))
			for _, char := range acct.Characters {
				opts = append(opts, huh.NewOption(char.DisplayName, char.AccountID))
			}

			var characterID string
			err = huh.NewSelect[string]().
				Title("Select character").
				Options(opts...).
				Value(&characterID).
				Run()
			if err != nil {
				return fmt.Errorf("selecting character: %w", err)
			}

			character := acct.Characters[slices.IndexFunc(acct.Characters, func(c auth.JagexCharacter) bool {
				return c.AccountID == characterID
			})]

			err = os.WriteFile(os.ExpandEnv(outputDestination), []byte(fmt.Sprintf(`
			JX_CHARACTER_ID=%s
			JX_SESSION_ID=%s
			JX_REFRESH_TOKEN=
			JX_DISPLAY_NAME=%s
			JX_ACCESS_TOKEN=
			`, character.AccountID, acct.Session, character.DisplayName)), 0644)
			if err != nil {
				return fmt.Errorf("writing credentials file: %w", err)
			}

			log.Info().Msg("Runelite is set! Closing this window.")
			time.Sleep(time.Second * 2)
			return nil
		},
	}
}
