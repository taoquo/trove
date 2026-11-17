package logging

import "regexp"

// redacted is what replaces a credential's value.
const redacted = "[redacted]"

// secretShapes are the forms a credential takes in a line of text. The list is narrow on
// purpose: each entry is a shape this app itself can produce -- an X cookie header, a
// model key -- so the file stays readable rather than becoming a wall of substitutions.
var secretShapes = []struct {
	pattern *regexp.Regexp
	replace string
}{
	// The two X cookies. They are the session: either one alone is enough to post as the
	// reader, so both are removed wherever a header or a URL carries them.
	{regexp.MustCompile(`(?i)\bct0=[^;\s"',}&]+`), "ct0=" + redacted},
	{regexp.MustCompile(`(?i)\bauth_token=[^;\s"',}&]+`), "auth_token=" + redacted},
	// Model keys, across the providers the app speaks to: OpenAI and Anthropic both use
	// an sk- prefix.
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}`), "sk-" + redacted},
	// A bearer token in an Authorization header.
	{regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/-]{12,}=*`), "Bearer " + redacted},
}

// redact replaces every credential shape in line.
//
// Replacements are idempotent, so a line that already holds the placeholder is unchanged
// by a second pass.
func redact(line string) string {
	for _, shape := range secretShapes {
		line = shape.pattern.ReplaceAllString(line, shape.replace)
	}
	return line
}
