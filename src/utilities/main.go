package main

import (
	"net/http"
	// SafeHarbor packages:
	"utilities/rest"  // we have to list this here or the pkg will not be built.
)

func main() {
	// We have to reference the rest package so this will compile.
	rest.CreateTCPRestContext("", "", 0, "", "",
		func(*http.Request, string) {})
}
