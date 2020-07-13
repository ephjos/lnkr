package lnkr

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
)

var db *bolt.DB
const BUCKET = "bindings"
const DB_FILE = "bolt/bindings.db"

func EnsureHttpDest(dest string) string {
	match, err := regexp.Match("^http[s]?://.*$",[]byte(dest))

	if err != nil {
		log.Fatal(err)
	}

	if !match {
		dest = "http://"+dest
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

	switch(r.Method) {
	case "GET":
		if dest, ok := GetDestFromSource(src); ok {
			log.Println("Redirecting to " + dest)
			http.Redirect(w,r,dest,http.StatusSeeOther)
			return
		} else {
			log.Println(src + " not bound")
			http.Redirect(w,r,"/",http.StatusNotFound)
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

		err = BindSrcDest(src, dest.Url)
		if err != nil {
			log.Println("failed to bind " + src + ", already bound")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(err.Error()))
			return
		}

		log.Println("bound " + src + " to " + dest.Url)
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
