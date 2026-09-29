//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Spicy-Bush/fider-tarkov-community/app/services/sqlstore/postgres/mediaowners"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: generate-media-schema")
		fmt.Fprintln(os.Stderr, "Print media-reference definitions for a new SQL migration.")
	}
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}

	if _, err := fmt.Print(mediaowners.UpdateSQL()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}