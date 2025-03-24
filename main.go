package main

import (
	"fmt"
	"os"

	"github.com/Emyrk/osrs-launcher/cmd"
)

func main() {
	err := cmd.New().RootCmd().Invoke().WithOS().Run()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// https://auth.jagex.com/game-session/v1

//const GAMESESSION_ACCOUNTS_ENDPOINT: &str = "https://auth.jagex.com/game-session/v1/accounts";
//const GAMESESSION_SESSION_ENDPOINT: &str = "https://auth.jagex.com/game-session/v1/sessions";
//const RS_PROFILE_ENDPOINT: &str = "https://secure.jagex.com/rs-profile/v1/profile";

// Profile api: https://secure.jagex.com/rs-profile/v1
// API: https://api.jagex.com/v1
// Auth_api: https://auth.jagex.com/game-session/v1
