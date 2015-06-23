package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"github.com/tschroed/flickr_to_photos/flickr"
	"github.com/tschroed/flickr_to_photos/workpool"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

var dbDumpPath = flag.String("flickr_db_dump",
	"/home/tschroed/tmp/flickr_dump.json", "Path to dump out all of the metadata pulled from Flickr via the API")

var photosDir = flag.String("flickr_photos_dir",
	"/home/tschroed/tmp/flickr_sync", "Path where Flickr photos will be copied")

func copyImage(url *url.URL, destFile string) {
	if err := os.MkdirAll(path.Dir(destFile), 0750); err != nil {
		log.Fatalf("Failed MkdirAll(%s): %v", path.Dir(destFile), err)
	}
	resp, err := http.Head(url.String())
	if err != nil {
		log.Fatalf("Failed to Head(%s): %v", url.String(), err)
	}
	fi, err := os.Stat(destFile)
	if err == nil {
		// Already got this, return.
		if fi.Size() == resp.ContentLength {
			log.Printf("%s up-to-date, skipping.\n", destFile)
			return
		}
		if err := os.Remove(destFile); err != nil {
			log.Fatalf("Failed to Remove(%s): %v", destFile, err)
		}
	}
	resp, err = http.Get(url.String())
	if err != nil {
		log.Fatalf("Failed to Get(%s): %v", url.String(), err)
	}
	defer resp.Body.Close()
	file, err := os.OpenFile(destFile, os.O_WRONLY|os.O_CREATE, 0640)
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		log.Fatalf("Failed to Copy: %v", url.String(), err)
	}
}

func extOf(path string) string {
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}

// This just blows up on err. Slightly work-inefficient in that we
// must complete a photoset before starting another.
func copyPhotos(photos []flickr.PhotoMetadata, destDir string) {
	wp := workpool.New(10, 0)
	wp.Start()
	for _, photo := range photos {
		url, err := photo.Url('o')
		if err != nil {
			log.Printf("Couldn't get URL for photo %v: %v. Skipping.", photo.Id, err)
			continue
		}
		destFile := fmt.Sprintf("%s/%v.%s", destDir, photo.Id, extOf(url.Path))
		wp.Add(func() {
			log.Printf("%v -> %v\n", url, destFile)
			copyImage(url, destFile)
			uploaded := time.Unix(photo.DateUpload, 0)
			if err := os.Chtimes(destFile, uploaded, uploaded); err != nil {
				log.Fatalf("Failed to Chtimes(%s, ...): %v", destFile, err)
			}
		})
	}
	wp.Close()
	wp.Join()
}

func main() {
	flag.Parse()
	// TODO(tschroed): Make this less duplicative
	tokenCred, err := flickr.LoadCachedCredentials()
	var c *flickr.Client
	if err != nil {
		log.Printf("Failed to load cached credentials: %#v", err)
		tokenCred, err = flickr.Authenticate()
		if err != nil {
			log.Fatalf("Failed to authenticate: %#v", err)
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
				log.Fatalf("Failed to authenticate: %#v", err)
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
	log.Printf("Got user: %s (%s)\n", user.Username, user.Id)

	sets, err := c.PhotosetsGetList()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Got %d photosets\n", len(sets))

	for _, set := range sets {
		photos, err := c.PhotosetsGetPhotos(set.Id)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Got %d photos in %s (%v)\n", len(photos), set.Title, set.Id)
		copyPhotos(photos, fmt.Sprintf("%s/sets/%v", *photosDir, set.Id))
	}
	/*
		notInSet, err := c.PhotosGetNotInSet()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Got %d photos not in sets\n", len(notInSet))
		if len(notInSet) > 0 {
			photo := notInSet[0]
			url, err := photo.Url('o')
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("A URL for one would be %s\n", url)
			copyImage(url, fmt.Sprintf("%s/%v/%v.%s", *photosDir, "not-in-set", photo.Id, extOf(url.Path)))
		}
	*/

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
	/*
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
	*/
}
