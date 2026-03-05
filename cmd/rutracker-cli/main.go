package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/minya/rutracker"
)

func main() {
	username := flag.String("u", "", "Rutracker username")
	password := flag.String("p", "", "Rutracker password")
	query := flag.String("q", "", "Search query")
	timeout := flag.Duration("timeout", 5*time.Second, "HTTP timeout")
	ipv6 := flag.Bool("6", false, "Force IPv6")
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: rutracker-cli -u <username> -p <password> [-q <query>] [-timeout 30s]")
		os.Exit(1)
	}

	start := time.Now()
	fmt.Fprintf(os.Stderr, "Authenticating (timeout=%s)...\n", *timeout)
	opts := []rutracker.Option{rutracker.WithTimeout(*timeout)}
	if *ipv6 {
		opts = append(opts, rutracker.WithIPv6())
	}
	client, err := rutracker.NewAuthenticatedRutrackerClient(*username, *password, opts...)
	fmt.Fprintf(os.Stderr, "Auth took %s\n", time.Since(start))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Auth failed: %v\n", err)
		os.Exit(1)
	}

	if *query == "" {
		fmt.Fprintln(os.Stderr, "Auth OK. No query specified.")
		return
	}

	start = time.Now()
	fmt.Fprintf(os.Stderr, "Searching: %s\n", *query)
	results, err := client.Find(*query)
	fmt.Fprintf(os.Stderr, "Search took %s\n", time.Since(start))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d results\n", len(results))
	for i, r := range results {
		fmt.Printf("%d. [%d seeders] %s (%.1f %s)\n", i+1, r.Seeders, r.Title, r.Size.Size, r.Size.Unit)
	}
}
