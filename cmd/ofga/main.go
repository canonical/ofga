package main

import (
	"fmt"

	hello "github.com/canonical/ofga"
	"github.com/canonical/ofga/internal/version"
)

func main() {
	fmt.Println(hello.Hello())
	fmt.Printf("Version: %s\n", version.Info())
}
