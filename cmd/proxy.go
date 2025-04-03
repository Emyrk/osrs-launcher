package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/coder/serpent"
)

func (r *Root) ProxyTest() *serpent.Command {
	var scriptPrint bool

	return &serpent.Command{
		Use:     "proxy-test",
		Aliases: []string{"proxy"},
		Options: serpent.OptionSet{
			{
				Name:          "script-print",
				Description:   "Print ip only.",
				Required:      false,
				Flag:          "script-print",
				FlagShorthand: "s",
				Value:         serpent.BoolOf(&scriptPrint),
			},
		},
		Middleware: serpent.Chain(r.LoggerMW(), UseProxy),
		Handler: func(i *serpent.Invocation) error {
			ctx := i.Context()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipconfig.io", nil)
			if err != nil {
				return fmt.Errorf("creating request: %w", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("sending request: %w", err)
			}

			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}

			out, _ := io.ReadAll(resp.Body)
			if scriptPrint {
				i.Stdout.Write(out)
			} else {
				log.Info().
					Str("ip", string(out)).
					Msg("Proxy test successful")
			}
			return nil
		},
	}
}

func UseProxy(next serpent.HandlerFunc) serpent.HandlerFunc {
	return func(i *serpent.Invocation) error {
		data, err := os.ReadFile("/etc/proxychains4.conf")
		if err == nil {
			// Do not use proxy if the user has set the no-proxy flag
			if no, _ := i.Command.Options.FlagSet().GetBool("no-proxy"); no {
				log.Info().Msg("Proxy settings file found, but disabled by user. No proxy will be used.")
				return next(i)
			}

			scanner := bufio.NewScanner(bytes.NewReader(data))
			for scanner.Scan() {
				line := scanner.Text()
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "#") {
					continue
				}

				if strings.HasPrefix(line, "socks") {
					parts := strings.Fields(line)
					if len(parts) < 3 {
						continue
					}
					// scheme://user:secret@ip:port
					var rawURL string
					// Includes auth
					if len(parts) >= 5 {
						scheme, ip, port, user, secret := parts[0], parts[1], parts[2], parts[3], parts[4]
						rawURL = fmt.Sprintf("%s://%s:%s@%s:%s", scheme, user, secret, ip, port)
					} else {
						scheme, ip, port := parts[0], parts[1], parts[2]
						rawURL = fmt.Sprintf("%s://%s:%s", scheme, ip, port)
					}

					pu, err := url.Parse(rawURL)
					if err != nil {
						log.Err(err).Str("url", rawURL).Msg("parsing proxy url, no proxy will be used")
						break
					}
					transport := &http.Transport{
						Proxy: http.ProxyURL(pu),
					}
					http.DefaultClient.Transport = transport
				}
			}
		}

		return next(i)
	}
}

func tsp(str string) string {
	return strings.TrimSpace(str)
}
