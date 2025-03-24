package cmd

import (
	"fmt"

	"github.com/Emyrk/osrs-launcher-auth/config"
	"github.com/charmbracelet/huh"
	"github.com/rs/zerolog/log"

	"github.com/coder/serpent"
)

func (r *Root) Delete() *serpent.Command {
	return &serpent.Command{
		Use:        "delete",
		Aliases:    []string{"del"},
		Options:    serpent.OptionSet{},
		Middleware: r.LoggerMW(),
		Handler: func(i *serpent.Invocation) error {
			root := config.DefaultDir().Init()
			all, err := root.Accounts()
			options := make([]huh.Option[string], 0, len(all))
			for _, account := range all {
				options = append(options, huh.Option[string]{
					Key:   account.Name(),
					Value: account.Name(),
				})
			}
			options = append(options)

			var sel string
			err = huh.NewSelect[string]().
				Title("Select Jagex account to authenticate").
				Options(options...).
				Value(&sel).
				Run()
			if err != nil {
				return fmt.Errorf("selecting: %w", err)
			}

			err = root.Account(sel).Delete()
			if err != nil {
				return fmt.Errorf("deleting account: %w", err)
			}

			log.Info().
				Str("account", sel).
				Msg("Account deleted")
			return nil
		},
	}
}
