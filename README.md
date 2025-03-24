This application replaces the Jagex launcher. It can authenticate into a jagex
account, select a character, and save the credentials into
`$HOME/.runelite/credentials.properties`. This is where RuneLite looks for
credentials by default on Linux.

# Installation

```shell
go install github.com/Emyrk/osrs-launcher@latest
```

# Usage

```shell
# The app needs to listen on port 80 for the oauth local callback
sudo setcap CAP_NET_BIND_SERVICE=+eip `which osrs-launcher`
osrs-launcher auth
```
