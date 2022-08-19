package main

import (
	"fmt"
	"os"

	"github.com/epinio/epinio/acceptance/helpers/auth"
)

func main() {
	args := os.Args[1:]
	if len(args) < 3 {
		panic("missing arguments [username, password, URL]")
	}

	username, password, domain := args[0], args[1], args[2]

	accessToken, err := auth.GetToken(domain, username, password)
	if err != nil {
		panic(err)
	}

	fmt.Print(accessToken)
}
