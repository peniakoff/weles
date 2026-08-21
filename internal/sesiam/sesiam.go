// Package sesiam helps deploy-time SES IAM identity checks and ARN construction.
package sesiam

import (
	"fmt"
	"strings"
)

// NormalizeIdentity trims an identity name or ARN and returns the identity
// resource name (domain or email), stripping a SES identity ARN prefix when present.
func NormalizeIdentity(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	const marker = ":identity/"
	if i := strings.LastIndex(raw, marker); i >= 0 && strings.Contains(raw, ":ses:") {
		return strings.TrimSpace(raw[i+len(marker):])
	}
	return raw
}

// SplitIdentities splits a comma-separated SES_IDENTITIES value into trimmed names.
func SplitIdentities(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if n := NormalizeIdentity(p); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// CoversFromEmail reports whether fromEmail is allowed by any listed SES identity
// (exact email match or domain identity matching the address domain).
func CoversFromEmail(identities []string, fromEmail string) bool {
	fromEmail = strings.TrimSpace(strings.ToLower(fromEmail))
	if fromEmail == "" {
		return false
	}
	at := strings.LastIndex(fromEmail, "@")
	if at <= 0 || at == len(fromEmail)-1 {
		return false
	}
	domain := fromEmail[at+1:]
	for _, id := range identities {
		id = strings.ToLower(NormalizeIdentity(id))
		if id == "" {
			continue
		}
		if id == fromEmail || id == domain {
			return true
		}
	}
	return false
}

// BuildARN builds a SES identity ARN for the commercial partition (arn:aws:ses:…).
// If identity is already an ARN containing ":ses:" and ":identity/", it is returned trimmed.
func BuildARN(region, account, identity string) (string, error) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return "", fmt.Errorf("identity is empty")
	}
	if strings.Contains(identity, ":ses:") && strings.Contains(identity, ":identity/") {
		return identity, nil
	}
	region = strings.TrimSpace(region)
	account = strings.TrimSpace(account)
	if region == "" || account == "" {
		return "", fmt.Errorf("region and account are required to build SES identity ARN")
	}
	name := NormalizeIdentity(identity)
	if name == "" {
		return "", fmt.Errorf("identity is empty")
	}
	return fmt.Sprintf("arn:aws:ses:%s:%s:identity/%s", region, account, name), nil
}

// BuildARNs builds SES identity ARNs for each identity name or ARN.
func BuildARNs(region, account string, identities []string) ([]string, error) {
	out := make([]string, 0, len(identities))
	seen := make(map[string]struct{}, len(identities))
	for _, id := range identities {
		arn, err := BuildARN(region, account, id)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[arn]; ok {
			continue
		}
		seen[arn] = struct{}{}
		out = append(out, arn)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no SES identities provided")
	}
	return out, nil
}
