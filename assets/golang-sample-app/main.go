package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

const INDEX = `<!DOCTYPE html>
<html>
  <head>
    <title>Powered By Paketo Buildpacks</title>
  </head>
  <body>
    <img style="display: block; margin-left: auto; margin-right: auto; width: 50%;" src="https://paketo.io/images/paketo-logo-full-color.png"></img>
  </body>
</html>`

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, INDEX)
	})

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}
