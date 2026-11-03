package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/minya/rutracker"
)

func main() {
	username := flag.String("u", "", "Rutracker username")
	passwordFlag := flag.String("p", "", "deprecated: Rutracker password (visible in ps and shell history); use "+passwordEnv+" or stdin")
	query := flag.String("q", "", "Search query")
	timeout := flag.Duration("timeout", 5*time.Second, "HTTP timeout")
	ipv6 := flag.Bool("6", false, "Force IPv6")
	flag.Parse()

	if *username == "" {
		fmt.Fprintln(os.Stderr, "Usage: "+passwordEnv+"=<password> rutracker-cli -u <username> [-q <query>] [-timeout 30s]")
		fmt.Fprintln(os.Stderr, "   or: <command printing the password> | rutracker-cli -u <username> ...")
		os.Exit(1)
	}
	password, err := readPassword(*passwordFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Password: %v\n", err)
		os.Exit(1)
	}

	start := time.Now()
	fmt.Fprintf(os.Stderr, "Authenticating (timeout=%s)...\n", *timeout)
	opts := []rutracker.Option{rutracker.WithTimeout(*timeout)}
	if *ipv6 {
		opts = append(opts, rutracker.WithIPv6())
	}
	client, err := rutracker.NewAuthenticatedRutrackerClient(*username, password, opts...)
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

// passwordEnv is the variable the password is read from; the bot uses the
// same name.
const passwordEnv = "TGT_RUTRACKER_PASSWORD"

// readPassword returns the password from the deprecated -p flag, else from
// TGT_RUTRACKER_PASSWORD, else from the first line of stdin. A command-line
// argument is readable by other local users through ps and lands in shell
// history; the environment and a pipe are not.
func readPassword(flagValue string) (string, error) {
	if flagValue != "" {
		fmt.Fprintln(os.Stderr, "warning: -p exposes the password in ps and shell history; set "+passwordEnv+" or pipe it on stdin instead")
		return flagValue, nil
	}
	if p := os.Getenv(passwordEnv); p != "" {
		return p, nil
	}
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprint(os.Stderr, "Rutracker password (input is echoed; prefer "+passwordEnv+"): ")
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("set %s or pass it on stdin", passwordEnv)
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", fmt.Errorf("empty password; set %s or pass it on stdin", passwordEnv)
	}
	return line, nil
}
