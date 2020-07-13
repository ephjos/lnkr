package main

import (
	"log"
	"net/http"

	"github.com/ephjos/lnkr"
)

func main() {
	http.Handle("/", lnkr.NewRouter())
	defer lnkr.Close()

	port := ":3333"
	log.Println("Listening on " + port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal(err)
	}
}

