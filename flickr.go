package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/garyburd/go-oauth/oauth"
	"io"
	"io/ioutil"
	"log"
	"net/url"
)

// API //

type Error struct {
	Code    int    `xml:"code,attr"`
	Message string `xml:"msg,attr"`
}

type Response struct {
	XMLName xml.Name `xml:"rsp"`
	Stat    string   `xml:"stat,attr"`
	Err     Error    `xml:"err"`
	Value   []byte   `xml:",innerxml"`
}

type PhotosetMetadata struct {
	XMLName       xml.Name `xml:"photoset"`
	Id            int64    `xml:"id,attr"`
	Primary       int64    `xml:"primary,attr"`
	Secret        string   `xml:"secret,attr"`
	Server        int      `xml:"server,attr"`
	Farm          int      `xml:"farm,attr"`
	Photos        int      `xml:"photos,attr"`
	Videos        int      `xml:"videos,attr"`
	CountViews    int      `xml:"count_views,attr"`
	CountComments int      `xml:"count_comments,attr"`
	CanComment    bool     `xml:"can_comment,attr"`
	DateCreate    string   `xml:"date_create,attr"`
	DateUpdate    string   `xml:"date_update,attr"`
	Title         string   `xml:"title"`
	Description   string   `xml:"description"`
}

func (c *client) PhotosetsGetList() ([]PhotosetMetadata, error) {
	value, err := c.call("flickr.photosets.getList", nil)
	if err != nil {
		return nil, err
	}
	var sets struct {
		Sets []PhotosetMetadata `xml:"photoset"`
	}
	if err := xml.Unmarshal(value, &sets); err != nil {
		return nil, err
	}
	return sets.Sets, nil
}

// // //

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

type client struct {
	creds *oauth.Credentials
}

func decodeResponse(body []byte) ([]byte, error) {
	var rsp Response
	if err := xml.Unmarshal([]byte(body), &rsp); err != nil {
		return nil, err
	}
	if rsp.Stat != "ok" {
		return nil, fmt.Errorf("Failed call: %#v", rsp.Err)
	}
	return rsp.Value, nil
}

func (c *client) call(method string, args url.Values) ([]byte, error) {
	if args == nil {
		args = url.Values{}
	}
	args["method"] = []string{method}
	resp, err := oauthClient.Get(nil, c.creds,
		"https://api.flickr.com/services/rest", args)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, resp.Body); err != nil {
		log.Fatal(err)
	}
	value, err := decodeResponse(buf.Bytes())
	if err != nil {
		return nil, err
	}
	return value, nil
}

func main() {
	tokenCred, err := Authenticate()
	if err != nil {
		log.Fatal(err)
	}

	c := client{creds: tokenCred}

	value, err := c.call("flickr.test.login", nil)
	if err != nil {
		log.Fatal(err)
	}
	var user struct {
		XMLName  xml.Name `xml:"user"`
		Id       string   `xml:"id,attr"`
		Username string   `xml:"username"`
	}
	if err := xml.Unmarshal(value, &user); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Got user: %s (%s)\n", user.Username, user.Id)

	sets, err := c.PhotosetsGetList()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Got %d photosets\n", len(sets))
}
