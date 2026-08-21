// Command deploycheck validates apps YAML against SES identities before deploy.
//
// Usage:
//
//	deploycheck -apps file.yaml -identities "example.com,other.example.com"
//	deploycheck -apps file.yaml -identities "..." -region eu-central-1 -account 123 -print-arns
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peniakoff/weles/internal/config"
	"github.com/peniakoff/weles/internal/sesiam"
)

func main() {
	appsPath := flag.String("apps", "", "path to apps registry YAML (required)")
	identitiesCSV := flag.String("identities", "", "comma-separated SES identities (domains or emails; ARNs accepted)")
	region := flag.String("region", "", "AWS region (required with -print-arns)")
	account := flag.String("account", "", "AWS account ID (required with -print-arns)")
	printARNs := flag.Bool("print-arns", false, "print comma-joined SES identity ARNs on success")
	flag.Parse()

	if strings.TrimSpace(*appsPath) == "" {
		fail("-apps is required")
	}
	if strings.TrimSpace(*identitiesCSV) == "" {
		fail("-identities is required")
	}

	data, err := os.ReadFile(*appsPath)
	if err != nil {
		fail("read apps: %v", err)
	}
	reg, err := config.ParseAppsYAML(data)
	if err != nil {
		fail("parse apps: %v", err)
	}

	identities := sesiam.SplitIdentities(*identitiesCSV)
	if len(identities) == 0 {
		fail("no SES identities after parsing -identities")
	}

	var uncovered []string
	for _, app := range reg.All() {
		if !sesiam.CoversFromEmail(identities, app.FromEmail) {
			uncovered = append(uncovered, fmt.Sprintf("%s fromEmail=%s", app.ID, app.FromEmail))
		}
	}
	if len(uncovered) > 0 {
		fail("fromEmail not covered by SES identities (%s):\n  - %s",
			strings.Join(identities, ", "),
			strings.Join(uncovered, "\n  - "))
	}

	if !*printARNs {
		return
	}
	if strings.TrimSpace(*region) == "" || strings.TrimSpace(*account) == "" {
		fail("-region and -account are required with -print-arns")
	}
	arns, err := sesiam.BuildARNs(*region, *account, identities)
	if err != nil {
		fail("build ARNs: %v", err)
	}
	fmt.Println(strings.Join(arns, ","))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "deploycheck: "+format+"\n", args...)
	os.Exit(1)
}
