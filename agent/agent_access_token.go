package agent

import (
	"bufio"
	"github.com/resourced/resourced/libstring"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func (a *Agent) setAccessTokens() error {
	a.AccessTokens = make([]string, 0)

	configDir := os.Getenv("RESOURCED_CONFIG_DIR")
	if configDir == "" {
		return nil
	}

	configDir = libstring.ExpandTildeAndEnv(configDir)
	accessTokensDir := path.Join(configDir, "access-tokens")

	tokenFiles, err := ioutil.ReadDir(accessTokensDir)
	if err != nil {
		return nil
	}

	for _, f := range tokenFiles {
		fullpath := path.Join(accessTokensDir, f.Name())

		file, err := os.Open(fullpath)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			tokensPerLine := strings.Split(scanner.Text(), ",")

			a.AccessTokens = append(a.AccessTokens, tokensPerLine...)
		}
	}

	return nil
}

// Check if a given access token is allowed.
func (a *Agent) IsAllowed(givenToken string) bool {
	// Allow all if there are no AccessTokens defined.
	if len(a.AccessTokens) == 0 {
		return true
	}

	for _, token := range a.AccessTokens {
		if token == givenToken {
			return true
		}
	}

	return false
}
