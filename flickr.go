package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/garyburd/go-oauth/oauth"
	"io"
	"io/ioutil"
	"log"
	"os"
	"net/url"
)

var oauthClient = oauth.Client{
	TemporaryCredentialRequestURI: "https://www.flickr.com/services/oauth/request_token",
	ResourceOwnerAuthorizationURI: "https://www.flickr.com/services/oauth/authorize",
	TokenRequestURI:               "https://www.flickr.com/services/oauth/access_token",
}

var credPath = flag.String("flickr_config", "/home/tschroed/flickr_config.json",
                           "Path to configuration file containing the application's credentials.")

// credentials should be json formatted like:
// {
//   "Token":"API key",
//   "Secret":"API secret"
// }
func readCredentials() error {
	b, err := ioutil.ReadFile(*credPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, &oauthClient.Credentials)
}

func Authenticate() (*oauth.Credentials, error) {
	if err := readCredentials(); err != nil {
		log.Fatal(err)
	}

	tempCred, err := oauthClient.RequestTemporaryCredentials(nil, "oob", nil)
	if err != nil {
		return nil, fmt.Errorf("RequestTemporaryCredentials: %v", err)
	}

	u := oauthClient.AuthorizationURL(tempCred, nil)

	fmt.Printf("1. Go to %s\n2. Authorize the application\n3. Enter verification code:\n", u)

	var code string
	fmt.Scanln(&code)

	tokenCred, _, err := oauthClient.RequestToken(nil, tempCred, code)
	if err != nil {
		return nil, fmt.Errorf("RequestToken: %v", err)
	}
	return tokenCred, nil
}

func main() {
	tokenCred, err := Authenticate()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := oauthClient.Get(nil, tokenCred,
	    "https://api.flickr.com/services/rest",
		url.Values{"method":{"flickr.test.login"}})
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		log.Fatal(err)
	}
}
