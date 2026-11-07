// Command index creates or updates the versioned Search index with Azure CLI credentials.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var searchHost = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,58}[a-z0-9]\.search\.windows\.net$`)

func indexURL(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || !searchHost.MatchString(u.Host) ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("endpoint must be an HTTPS Azure AI Search service URL")
	}
	return "https://" + u.Host + "/indexes/questoes-v1?api-version=2024-07-01", nil
}

func run(endpoint, schema string) error {
	target, err := indexURL(endpoint)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(schema)
	if err != nil {
		return fmt.Errorf("resolve schema: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("schema must be a regular file")
	}
	cmd := exec.Command("az", "rest", "--method", "put", "--uri", target,
		"--resource", "https://search.azure.com", "--body", "@"+path, "--only-show-errors")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create or update index: %w", err)
	}
	return nil
}

func main() {
	endpoint := flag.String("endpoint", "", "Azure AI Search service URL")
	schema := flag.String("schema", "index/questoes-v1.json", "versioned index schema")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	if err := run(*endpoint, *schema); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
