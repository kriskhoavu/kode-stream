package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"plan-manager/internal/app"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		port := fs.Int("port", 0, "localhost port, defaults to 4317 or PLAN_MANAGER_PORT")
		if err := fs.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		server, err := app.NewServer(*port)
		if err != nil {
			log.Fatal(err)
		}
		if err := server.Serve(); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: plan-manager serve [-port 4317]")
}
