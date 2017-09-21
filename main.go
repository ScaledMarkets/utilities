package utilities

import (
	// SafeHarbor packages:
	"utilities"  // we have to list this here or the pkg will not be built.
)

func main() {
	// We have to reference the rest package so this will compile.
	rest.CreateTCPRestContext("", "", 0, "", "", nil,
		func(*http.Request, string) {})
}
