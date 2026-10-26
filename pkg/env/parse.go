package env

import (
	"fmt"
	"strings"
)

var unescaper = strings.NewReplacer(`\\`, `\`, `\"`, `"`, `\n`, "\n", `\r`, "\r", `\t`, "\t")

// Parse decodes dotenv bytes into a key-value map. Errors report line numbers
// only, never line content, so secrets can't leak into logs.
func Parse(data []byte) (map[string]string, error) {
	vars := make(map[string]string)
	src := strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\xef\xbb\xbf") // strip UTF-8 BOM

	for lineNo := 1; src != ""; lineNo++ {
		line, rest, hasMore := strings.Cut(src, "\n")
		src = rest

		line = strings.TrimLeft(line, " \t")
		if line == "" || line[0] == '#' {
			continue
		}
		if after, ok := strings.CutPrefix(line, "export"); ok && strings.TrimLeft(after, " \t") != after {
			line = strings.TrimLeft(after, " \t")
		}

		key, val, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", lineNo)
		}
		val = strings.TrimLeft(val, " \t")

		if val == "" || !strings.ContainsRune("\"'`", rune(val[0])) {
			// Unquoted: a '#' preceded by whitespace starts an inline comment.
			for i := 1; i < len(val); i++ {
				if val[i] == '#' && (val[i-1] == ' ' || val[i-1] == '\t') {
					val = val[:i]
					break
				}
			}
			vars[key] = strings.TrimRight(val, " \t")
			continue
		}

		// Quoted values may span lines.
		quote, body := val[0], val[1:]
		if hasMore {
			body += "\n" + src
		}
		end := closingQuote(body, quote)
		if end < 0 {
			return nil, fmt.Errorf("line %d: unterminated %c-quoted value for %s", lineNo, quote, key)
		}
		value := body[:end]
		openLine := lineNo
		lineNo += strings.Count(value, "\n")
		trailing, rest, _ := strings.Cut(body[end+1:], "\n")
		if t := strings.TrimLeft(trailing, " \t"); t != "" && t[0] != '#' {
			return nil, fmt.Errorf("line %d: unexpected text after closing quote of %s (opened on line %d)", lineNo, key, openLine)
		}
		src = rest
		if quote == '"' {
			value = unescaper.Replace(value)
		}
		vars[key] = value
	}
	return vars, nil
}

func closingQuote(s string, quote byte) int {
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && quote == '"':
			i++
		case s[i] == quote:
			return i
		}
	}
	return -1
}
