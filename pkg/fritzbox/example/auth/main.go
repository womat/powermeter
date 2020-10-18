package main

import (
	"fmt"
	"log"

	"github.com/philippfranke/go-fritzbox/fritzbox"
)

func main() {
	fmt.Printf("Connect to local FRITZ!Box! \n \n")

	c := fritzbox.NewClient(nil)

	var username, password string

	fmt.Print("Enter username: ")
	fmt.Scan(&username)
	fmt.Println("\u2757  Caution: Reading password from STDIN with echoing \u2757")
	fmt.Print("Enter password: ")
	fmt.Scan(&password)

	if err := c.Auth(username, password); err != nil {
		log.Fatalf("Auth failed: %v", err)
	}

	fmt.Printf("Successfully logged in! \n \t Session ID: %s \n", c)
}
