package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"plan-manager/internal/server"
	"plan-manager/internal/system"
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
		server, err := server.NewServer(*port)
		if err != nil {
			log.Fatal(err)
		}
		if err := server.Serve(); err != nil {
			log.Fatal(err)
		}
	case "doctor":
		if err := runDoctor(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	provider := fs.String("provider", "", "target provider: github or bitbucket")
	format := fs.String("format", "text", "output format: text or json")
	repo := fs.String("repo", "", "repository path or remote URL")
	strict := fs.Bool("strict", false, "treat warnings as non-zero exit")
	port := fs.Int("port", 0, "optional localhost port check")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	fmtValue := strings.ToLower(strings.TrimSpace(*format))
	if fmtValue != "text" && fmtValue != "json" {
		return fmt.Errorf("unsupported --format %q (expected text or json)", *format)
	}

	svc := system.NewDoctorService()
	result := svc.Run(system.Options{
		Provider: strings.TrimSpace(*provider),
		Repo:     strings.TrimSpace(*repo),
		Port:     *port,
		Strict:   *strict,
	})

	if fmtValue == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(os.Stdout, "plan-manager doctor")
		fmt.Fprintln(os.Stdout, "")
		for _, check := range result.Checks {
			status := strings.ToUpper(string(check.Status))
			fmt.Fprintf(os.Stdout, "%s %-20s %s\n", status, check.ID, check.Message)
			for _, item := range check.Remediation {
				fmt.Fprintf(os.Stdout, "  fix: %s\n", item)
			}
		}
		fmt.Fprintln(os.Stdout, "")
		fmt.Fprintf(os.Stdout, "Result: %d passed, %d failed, %d warnings\n", result.Summary.Passed, result.Summary.Failed, result.Summary.Warnings)
	}

	os.Exit(result.ExitCode(*strict))
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  plan-manager serve [-port 4317]")
	fmt.Fprintln(os.Stderr, "  plan-manager doctor [--provider github|bitbucket] [--repo <path-or-url>] [--format text|json] [--strict] [--port <n>]")
}
