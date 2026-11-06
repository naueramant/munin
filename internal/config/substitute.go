package config

import (
	"os"
	"regexp"
)

// varSubstitutionPattern matches ${var} and ${var:=default} references.
var varSubstitutionPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)(:=([^}]*))?\}`)

// substituteEnvVars expands ${var} and ${var:=default} references using
// environment variables. For ${var} an unset variable expands to an empty
// string; for ${var:=default} the default is used when the variable is unset
// or empty.
func substituteEnvVars(data []byte) []byte {
	out, _ := substituteEnvVarsReport(data)
	return out
}

// substituteEnvVarsReport is substituteEnvVars that also returns the names of
// ${var} references that were unset and had no default, in order of first use.
func substituteEnvVarsReport(data []byte) ([]byte, []string) {
	var unset []string
	seen := map[string]bool{}

	out := varSubstitutionPattern.ReplaceAllFunc(data, func(match []byte) []byte {
		groups := varSubstitutionPattern.FindSubmatch(match)
		name := string(groups[1])
		value, ok := os.LookupEnv(name)
		if (!ok || value == "") && len(groups[2]) > 0 {
			return groups[3]
		}
		if !ok && !seen[name] {
			seen[name] = true
			unset = append(unset, name)
		}
		return []byte(value)
	})

	return out, unset
}
