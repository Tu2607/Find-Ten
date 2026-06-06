package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"find-ten-game/internal/api"
)

func main() {
	addr := flag.String("addr", ":8080", "server listen address")
	flag.Parse()

	server := api.NewServer()
	fmt.Fprintf(os.Stdout, "listening on %s\n", *addr)
	if err := http.ListenAndServe(*addr, server); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
