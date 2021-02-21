package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"strings"
)

// Command line - mostly for debug and tools

var (
	jwt=flag.String("jwt", "", "JWT to decode")
)

func main() {
	flag.Parse()
	if *jwt != "" {
		decode(*jwt)
		return
	}


}

func decode(jwt string) {
	parts := strings.Split(jwt, ".")
	p1b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(string(p1b))
}
