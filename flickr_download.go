package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/tschroed/flickr_to_photos/flickr"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
)

var dbDumpPath = flag.String("flickr_db_dump",
	"/home/tschroed/tmp/flickr_dump.json", "Path to dump out all of the metadata pulled from Flickr via the API")

var photosDir = flag.String("flickr_photos_dir",
	"/home/tschroed/tmp/flickr_sync", "Path where Flickr photos will be copied")

func copyImage(url *url.URL, destFile string) {
	if err := os.MkdirAll(path.Dir(destFile), 0750); err != nil {
		panic(err)
	}
	resp, err := http.Head(url.String())
	if err != nil {
		panic(err)
	}
	fi, err := os.Stat(destFile)
	if err == nil {
		// Already got this, return.
		if fi.Size() == resp.ContentLength {
			return
		}
		if err := os.Remove(destFile); err != nil {
			panic(err)
		}
	}
	resp, err = http.Get(url.String())
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	file, err := os.OpenFile(destFile, os.O_WRONLY | os.O_CREATE, 0640)
	defer file.Close()
	io.Copy(file, resp.Body)
}

func main() {
	// TODO(tschroed): Make this less duplicative
	tokenCred, err := flickr.LoadCachedCredentials()
	var c *flickr.Client
	if err != nil {
		log.Printf("Failed to load cached credentials: %#v", err)
		tokenCred, err = flickr.Authenticate()
		if err != nil {
			log.Fatal("Failed to authenticate: %#v", err)
		}
		c = flickr.New(&flickr.DefaultOAuthClient, tokenCred)
		flickr.SaveCachedCredentials(tokenCred)
	} else {
		c = flickr.New(&flickr.DefaultOAuthClient, tokenCred)
		log.Printf("Loaded cached credentials")
		_, err := c.Call("flickr.test.login", nil)
		if err != nil {
			log.Printf("Failed to make auth call: %#v", err)
			tokenCred, err = flickr.Authenticate()
			if err != nil {
				log.Fatal("Failed to authenticate: %#v", err)
			}
			c = flickr.New(&flickr.DefaultOAuthClient, tokenCred)
			flickr.SaveCachedCredentials(tokenCred)
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
	fmt.Printf("Got %d photos in %s (%v)\n", len(inSetZero), sets[0].Title, sets[0].Id)
	if len(inSetZero) > 0 {
		url, err := inSetZero[0].Url('o')
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("A URL for one would be %s\n", url)
	}

	notInSet, err := c.PhotosGetNotInSet()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Got %d photos not in sets\n", len(notInSet))
	if len(notInSet) > 0 {
		url, err := notInSet[0].Url('o')
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("A URL for one would be %s\n", url)
		copyImage(url, "/tmp/out.jpg")
	}

	file, err := os.OpenFile(*dbDumpPath, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	bytes, err := json.MarshalIndent(sets, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	file.Write(bytes)
	bytes, err = json.MarshalIndent(inSetZero, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	file.Write(bytes)
	bytes, err = json.MarshalIndent(notInSet, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	file.Write(bytes)
}
