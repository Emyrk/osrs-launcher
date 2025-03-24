package config

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Emyrk/osrs-launcher/auth"
	"github.com/kirsle/configdir"
	"golang.org/x/xerrors"
)

type Root string

// mustNotBeEmpty prevents us from accidentally writing configuration to the
// current directory. This is primarily valuable in development, where we may
// accidentally use an empty root.
func (r Root) mustNotEmpty() {
	if r == "" {
		panic("config root must not be empty")
	}
}

func (r Root) Init() Root {
	_ = os.MkdirAll(string(r), 0o700)
	return r
}

func (r Root) Account(name string) Account {
	return Account(filepath.Join(string(r), name))
}

func (r Root) Accounts() ([]Account, error) {
	r.mustNotEmpty()
	entries, err := os.ReadDir(string(r))
	if err != nil {
		return nil, err
	}

	var accounts []Account
	for _, entry := range entries {
		if entry.IsDir() {
			accounts = append(accounts, Account(filepath.Join(string(r), entry.Name())))
		}
	}
	return accounts, nil
}

type Account string

func (a Account) Delete() error {
	return os.RemoveAll(string(a))
}

func (a Account) Name() string {
	return filepath.Base(string(a))
}

func (a Account) Token() (auth.JagexAccountAuth, error) {
	var token auth.JagexAccountAuth
	err := File(filepath.Join(string(a), "token")).ReadJSON(&token)
	return token, err
}

func (a Account) SaveToken(token *auth.JagexAccountAuth) error {
	return File(filepath.Join(string(a), "token")).WriteJSON(token)
}

// File provides convenience methods for interacting with *os.File.
type File string

func (f File) Exists() bool {
	if f == "" {
		return false
	}
	_, err := os.Stat(string(f))
	return err == nil
}

// Delete deletes the file.
func (f File) Delete() error {
	if f == "" {
		return xerrors.Errorf("empty file path")
	}
	return os.Remove(string(f))
}

// Write writes the string to the file.
func (f File) Write(s string) error {
	if f == "" {
		return xerrors.Errorf("empty file path")
	}
	return write(string(f), 0o600, []byte(s))
}

func (f File) WriteJSON(obj interface{}) error {
	if f == "" {
		return xerrors.Errorf("empty file path")
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return write(string(f), 0o600, data)
}

// Read reads the file to a string. All leading and trailing whitespace
// is removed.
func (f File) Read() (string, error) {
	if f == "" {
		return "", xerrors.Errorf("empty file path")
	}
	byt, err := read(string(f))
	return strings.TrimSpace(string(byt)), err
}

func (f File) ReadJSON(into interface{}) error {
	if f == "" {
		return xerrors.Errorf("empty file path")
	}
	byt, err := read(string(f))
	if err != nil {
		return err
	}
	return json.Unmarshal(byt, into)
}

// open opens a file in the configuration directory,
// creating all intermediate directories.
func open(path string, flag int, mode os.FileMode) (*os.File, error) {
	err := os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		return nil, err
	}

	return os.OpenFile(path, flag, mode)
}

func write(path string, mode os.FileMode, dat []byte) error {
	fi, err := open(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, mode)
	if err != nil {
		return err
	}
	defer fi.Close()
	_, err = fi.Write(dat)
	return err
}

func read(path string) ([]byte, error) {
	fi, err := open(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer fi.Close()
	return io.ReadAll(fi)
}

func DefaultDir() Root {
	configDir := configdir.LocalConfig("osrs-launcher")
	return Root(configDir)
}
