package lnkr

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
)

var db *bolt.DB

const BUCKET = "bindings"
const DB_FILE = "bolt/bindings.db"
var REQ = map[string]string {
	"gh": "github.com/ephjos",
}

func StoreMap(m map[string]string) {
	for k,v := range m {
		err := db.Update(func(tx *bolt.Tx) error {
			b := CreateBindingsBucket(tx)
			err := b.Put([]byte(k), []byte(EnsureHttpDest(v)))
			return err
		})

		if err != nil {
			log.Fatal(err)
		}
	}
}

func EnsureHttpDest(dest string) string {
	match, err := regexp.Match("^http[s]?://.*$", []byte(dest))

	if err != nil {
		log.Fatal(err)
	}

	if !match {
		dest = "http://" + dest
	}

	return dest
}

func CreateBindingsBucket(tx *bolt.Tx) *bolt.Bucket {
	if !tx.Writable() {
		log.Fatal("tx is not writable")
	}

	b, err := tx.CreateBucketIfNotExists([]byte(BUCKET))
	if err != nil {
		log.Fatal(err)
	}

	return b
}

func BindSrcDest(src, dest string) error {
	if _, ok := GetDestFromSource(src); ok {
		return fmt.Errorf("src already bound")
	}

	err := db.Update(func(tx *bolt.Tx) error {
		b := CreateBindingsBucket(tx)
		err := b.Put([]byte(src), []byte(EnsureHttpDest(dest)))
		return err
	})

	return err
}

func GetDestFromSource(src string) (string, bool) {
	var res string
	err := db.Update(func(tx *bolt.Tx) error {
		b := CreateBindingsBucket(tx)
		res = string(b.Get([]byte(src)))
		return nil
	})

	if err != nil || res == "" {
		return "", false
	}

	return res, true
}

func UrlShortener(w http.ResponseWriter, r *http.Request) {
	src := mux.Vars(r)["src"]

	var eSrc, cSrc, cDest string
	var err error
	if eSrc, err = url.PathUnescape(src); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	cSrc = url.PathEscape(eSrc)

	switch r.Method {
		case "GET":
			if dest, ok := GetDestFromSource(cSrc); ok {
				log.Printf("redirecting to %s\n", dest)
				http.Redirect(w, r, dest, http.StatusSeeOther)
				return
			} else {
				log.Printf("%s not bound\n", cSrc)
				http.Redirect(w, r, "/", http.StatusNotFound)
				return
			}

		case "POST":
			var dest struct {
				Url string
			}

			err := json.NewDecoder(r.Body).Decode(&dest)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}

			cDest = dest.Url
			log.Printf("Trying to bind %s to %s\n", cSrc, cDest)
			_, err = http.Get(fmt.Sprintf("http://%s", cDest))
			if err != nil {
				log.Println(err)
				log.Printf("%s is not a valid url\n", cDest)
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}

			err = BindSrcDest(cSrc, dest.Url)
			if err != nil {
				log.Printf("failed to bind %s, already bound\n", cSrc)
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte(err.Error()))
				return
			}

			log.Printf("%s bound to %s\n", cSrc, cDest)
			w.WriteHeader(http.StatusCreated)
			return

	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 - Not Found (Unknown link)"))
	}
}

func NewRouter() *mux.Router {
	var err error
	db, err = bolt.Open(DB_FILE, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	StoreMap(REQ)

	FileHandler := http.FileServer(http.Dir("./static"))

	r := mux.NewRouter()
	r.HandleFunc("/", FileHandler.ServeHTTP)
	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	r.HandleFunc("/{src}", UrlShortener)

		return r
}

func Close() {
	db.Close()
}
