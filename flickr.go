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
	"strconv"
	"strings"
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

func (c *client) Call(method string, args url.Values) ([]byte, error) {
	log.Printf("Calling %s\n", method)
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
	value, err := c.Call("flickr.photosets.getList", nil)
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

type PhotoMetadata struct {
	XMLName  xml.Name `xml:"photo"`
	Id       int64    `xml:"id,attr"`
	Farm     int      `xml:"farm,attr"`
	Owner    string   `xml:"owner,attr"`
	Secret   string   `xml:"secret,attr"`
	Server   int      `xml:"server,attr"`
	Title    string   `xml:"title,attr"`
	IsPublic bool     `xml:"ispublic,attr"`
}

func (m *PhotoMetadata) Url(size string) (*url.URL, error) {
	var urlString string
	if size == "" {
		urlString = fmt.Sprintf("https://farm%d.staticflickr.com/%d/%d_%s.jpg",
			m.Farm, m.Server, m.Id, m.Secret)
	} else {
		urlString = fmt.Sprintf("https://farm%d.staticflickr.com/%d/%d_%s_%c.jpg",
			m.Farm, m.Server, m.Id, m.Secret, size)
	}
	return url.Parse(urlString)
}

// TODO(trevors): Paginated calls should be generic. "Here's a call
// and a struct type, keep making it until we run out of pages. Give
// me back the results"
func (c *client) PhotosGetNotInSet(extras []string) ([]PhotoMetadata, error) {
	args := url.Values{
		"per_page": {"500"},
	}
	if extras != nil {
		args["extras"] = []string{strings.Join(extras, ",")}
	}

	photos := make([]PhotoMetadata, 0)
	for lastPage, curPage := 1, 1; curPage <= lastPage; curPage++ {
		args["page"] = []string{strconv.Itoa(curPage)}

		value, err := c.Call("flickr.photos.getNotInSet", args)
		if err != nil {
			return nil, err
		}
		var p struct {
			Photos []PhotoMetadata `xml:"photo"`
			Pages  int             `xml:"pages,attr"`
		}
		if err := xml.Unmarshal(value, &p); err != nil {
			return nil, err
		}
		photos = append(photos, p.Photos...)
		lastPage = p.Pages
	}
	return photos, nil
}

func (c *client) PhotosetsGetPhotos(photoset_id int64) ([]PhotoMetadata, error){
	args := url.Values{
		"per_page": {"500"},
		"photoset_id": {strconv.FormatInt(photoset_id, 10)},
	}

	photos := make([]PhotoMetadata, 0)
	for lastPage, curPage := 1, 1; curPage <= lastPage; curPage++ {
		args["page"] = []string{strconv.Itoa(curPage)}

		value, err := c.Call("flickr.photosets.GetPhotos", args)
		if err != nil {
			return nil, err
		}
		var p struct {
			Photos []PhotoMetadata `xml:"photo"`
			Pages  int             `xml:"pages,attr"`
		}
		if err := xml.Unmarshal(value, &p); err != nil {
			return nil, err
		}
		photos = append(photos, p.Photos...)
		lastPage = p.Pages
	}
	return photos, nil
}

// // //

var oauthClient = oauth.Client{
	TemporaryCredentialRequestURI: "https://www.flickr.com/services/oauth/request_token",
	ResourceOwnerAuthorizationURI: "https://www.flickr.com/services/oauth/authorize",
	TokenRequestURI:               "https://www.flickr.com/services/oauth/access_token",
}

var credPath = flag.String("flickr_config", "/home/tschroed/flickr_config.json",
	"Path to configuration file containing the application's credentials.")

var credCachePath = flag.String("flickr_creds", "/home/tschroed/flickr_creds.json",
	"Path to configuration file containing the application's cached credentials.")

// credentials should be json formatted like:
// {
//   "Token":"API key",
//   "Secret":"API secret"
// }
func readCredentials(path string, creds *oauth.Credentials) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, creds)
}

func LoadCachedCredentials() (*oauth.Credentials, error) {
	if err := readCredentials(*credPath, &oauthClient.Credentials); err != nil {
		log.Fatal(err)
	}
	creds := &oauth.Credentials{}
	if err := readCredentials(*credCachePath, creds); err != nil {
		return nil, err
	}
	return creds, nil
}

func SaveCachedCredentials(creds *oauth.Credentials) error {
	bytes, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(*credCachePath, bytes, 0600)
}

func Authenticate() (*oauth.Credentials, error) {
	if err := readCredentials(*credPath, &oauthClient.Credentials); err != nil {
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

func main() {
	// TODO(tschroed): Make this less duplicative
	tokenCred, err := LoadCachedCredentials()
	var c client
	if err != nil {
		log.Printf("Failed to load cached credentials: %#v", err)
		tokenCred, err = Authenticate()
		if err != nil {
			log.Fatal("Failed to authenticate: %#v", err)
		}
		c.creds = tokenCred
		SaveCachedCredentials(tokenCred)
	} else {
		c.creds = tokenCred
		log.Printf("Loaded cached credentials")
		_, err := c.Call("flickr.test.login", nil)
		if err != nil {
			log.Printf("Failed to make auth call: %#v", err)
			tokenCred, err = Authenticate()
			if err != nil {
				log.Fatal("Failed to authenticate: %#v", err)
			}
			c.creds = tokenCred
			SaveCachedCredentials(tokenCred)
		}
	}

	value, err := c.Call("flickr.test.login", nil)
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

	inSetZero, err := c.PhotosetsGetPhotos(sets[0].Id)
	fmt.Printf("Got %d photos in %s\n", len(inSetZero), sets[0].Title)

	notInSet, err := c.PhotosGetNotInSet([]string{"url_o"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Got %d photos not in sets\n", len(notInSet))
}
