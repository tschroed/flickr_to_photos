package flickr

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

func (c *Client) Call(method string, args url.Values) ([]byte, error) {
	log.Printf("Calling %s\n", method)
	if args == nil {
		args = url.Values{}
	}
	args["method"] = []string{method}
	resp, err := c.oauthClient.Get(nil, c.oauthCreds,
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

func (c *Client) PhotosetsGetList() ([]PhotosetMetadata, error) {
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
	XMLName    xml.Name `xml:"photo"`
	Id         int64    `xml:"id,attr"`
	Farm       int      `xml:"farm,attr"`
	Owner      string   `xml:"owner,attr"`
	Secret     string   `xml:"secret,attr"`
	Server     int      `xml:"server,attr"`
	Title      string   `xml:"title,attr"`
	IsPublic   bool     `xml:"ispublic,attr"`
	OUrl       string   `xml:"url_o,attr"`
	DateTaken  string   `xml:"datetaken,attr"`
	DateUpload int64    `xml:"dateupload,attr"`
}

func (m *PhotoMetadata) Url(size rune) (*url.URL, error) {
	var urlString string
	switch size {
	case 0:
		urlString = fmt.Sprintf("https://farm%d.staticflickr.com/%d/%d_%s.jpg",
			m.Farm, m.Server, m.Id, m.Secret)
	case 'o':
		if len(m.OUrl) == 0 {
			return nil, fmt.Errorf("No original URL for %v", m.Id)
		}
		urlString = m.OUrl
	default:
		urlString = fmt.Sprintf("https://farm%d.staticflickr.com/%d/%d_%s_%c.jpg",
			m.Farm, m.Server, m.Id, m.Secret, size)
	}
	return url.Parse(urlString)
}

// Some photos calls are fundamentally the same but have pagination
// needs so we handle them in one place.
func (c *Client) paginatedPhotosCall(method string, args url.Values) ([]PhotoMetadata, error) {
	args["per_page"] = []string{"500"}
	args["extras"] = []string{"url_o,date_upload,date_taken"}
	photos := make([]PhotoMetadata, 0)
	for lastPage, curPage := 1, 1; curPage <= lastPage; curPage++ {
		args["page"] = []string{strconv.Itoa(curPage)}

		value, err := c.Call(method, args)
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

func (c *Client) PhotosGetNotInSet() ([]PhotoMetadata, error) {
	args := url.Values{}
	return c.paginatedPhotosCall("flickr.photos.getNotInSet", args)
}

func (c *Client) PhotosetsGetPhotos(photoset_id int64) ([]PhotoMetadata, error) {
	args := url.Values{
		"photoset_id": {strconv.FormatInt(photoset_id, 10)},
	}
	return c.paginatedPhotosCall("flickr.photosets.getPhotos", args)
}

// // //

var DefaultOAuthClient = oauth.Client{
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

// TODO(trevors): Touches module scoped oauthClient
func LoadCachedCredentials() (*oauth.Credentials, error) {
	if err := readCredentials(*credPath, &DefaultOAuthClient.Credentials); err != nil {
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

// TODO(trevors): Touches module scoped oauthClient
func Authenticate() (*oauth.Credentials, error) {
	if err := readCredentials(*credPath, &DefaultOAuthClient.Credentials); err != nil {
		log.Fatal(err)
	}
	tempCred, err := DefaultOAuthClient.RequestTemporaryCredentials(nil, "oob", nil)
	if err != nil {
		return nil, fmt.Errorf("RequestTemporaryCredentials: %v", err)
	}

	u := DefaultOAuthClient.AuthorizationURL(tempCred, nil)

	fmt.Printf("1. Go to %s\n2. Authorize the application\n3. Enter verification code:\n", u)

	var code string
	fmt.Scanln(&code)

	tokenCred, _, err := DefaultOAuthClient.RequestToken(nil, tempCred, code)
	if err != nil {
		return nil, fmt.Errorf("RequestToken: %v", err)
	}
	return tokenCred, nil
}

type Client struct {
	oauthClient *oauth.Client
	oauthCreds  *oauth.Credentials
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

func New(oauthClient *oauth.Client, oauthCreds *oauth.Credentials) *Client {
	return &Client{
		oauthClient: oauthClient,
		oauthCreds:  oauthCreds,
	}
}
