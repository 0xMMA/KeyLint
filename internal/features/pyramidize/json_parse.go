package pyramidize

import (
	"encoding/json"
	"strings"
)

// stripFences removes ```json``` and ``` code fences from a string and trims surrounding whitespace.
// Many LLMs wrap their JSON output in markdown code fences despite being instructed not to.
func stripFences(s string) string {
	s = strings.TrimSpace(s)

	// Remove opening fence variants: ```json, ```JSON, ```
	if strings.HasPrefix(s, "```") {
		// Find the end of the opening fence line
		newline := strings.Index(s, "\n")
		if newline != -1 {
			s = s[newline+1:]
		} else {
			// Entire content is just the fence marker — strip it
			s = strings.TrimPrefix(s, "```json")
			s = strings.TrimPrefix(s, "```JSON")
			s = strings.TrimPrefix(s, "```")
		}
	}

	// Remove closing fence
	if strings.HasSuffix(strings.TrimSpace(s), "```") {
		idx := strings.LastIndex(s, "```")
		if idx != -1 {
			s = s[:idx]
		}
	}

	return strings.TrimSpace(s)
}

// repairJSONStrings fixes a common LLM defect where double-quote characters inside a
// JSON string value are not escaped.  For example:
//
//	"fullDocument": "...he said "hello" and left..."
//
// becomes:
//
//	"fullDocument": "...he said \"hello\" and left..."
//
// Strategy: walk the JSON byte-by-byte tracking whether we are inside a string value.
// When we encounter a '"' that would close the current string but is NOT followed by a
// structural character (':', ',', '}', ']', or end-of-meaningful-content), we assume
// it is a stray content quote and escape it.
func repairJSONStrings(s string) string {
	// Work on runes to handle multi-byte characters safely.
	runes := []rune(s)
	n := len(runes)

	var out []rune
	out = make([]rune, 0, n+32)

	// States
	inString := false   // inside a JSON string
	isValue := false    // the string is a value (not a key)
	afterColon := false // just saw ':' at top level

	i := 0
	for i < n {
		ch := runes[i]

		if !inString {
			out = append(out, ch)
			switch ch {
			case '"':
				inString = true
				// A string is a value when the last non-whitespace non-brace non-bracket
				// char outside a string was ':'.
				isValue = afterColon
				afterColon = false
			case ':':
				afterColon = true
			case ' ', '\t', '\n', '\r':
				// keep afterColon state across whitespace
			default:
				if ch != ':' {
					afterColon = false
				}
			}
			i++
			continue
		}

		// Inside a string.
		if ch == '\\' {
			// Already-escaped sequence — copy verbatim (char + next char).
			out = append(out, ch)
			i++
			if i < n {
				out = append(out, runes[i])
				i++
			}
			continue
		}

		if ch != '"' {
			out = append(out, ch)
			i++
			continue
		}

		// ch == '"' — need to decide: is this a legitimate closing quote or a
		// stray content quote that should be escaped?
		if !isValue {
			// Keys must not contain unescaped quotes — treat as real close.
			out = append(out, ch)
			inString = false
			afterColon = false
			i++
			continue
		}

		// Look ahead: skip any whitespace after this '"' and check the next char.
		j := i + 1
		for j < n && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n' || runes[j] == '\r') {
			j++
		}

		var nextMeaningful rune
		if j < n {
			nextMeaningful = runes[j]
		}

		isStructural := nextMeaningful == ':' || nextMeaningful == ',' ||
			nextMeaningful == '}' || nextMeaningful == ']' || j >= n

		if isStructural {
			// Legitimate closing quote.
			out = append(out, ch)
			inString = false
			afterColon = false
		} else {
			// Stray content quote — escape it.
			out = append(out, '\\', '"')
		}
		i++
	}

	return string(out)
}

// unmarshalRobust strips markdown fences from data then unmarshals it into v.
// If standard decoding fails (e.g. unescaped quotes from LLM output), it attempts
// a heuristic repair before giving up.
func unmarshalRobust(data string, v any) error {
	clean := stripFences(data)

	// Trim any leading non-JSON content before the first '{'.
	if idx := strings.Index(clean, "{"); idx > 0 {
		clean = clean[idx:]
	}

	// First attempt: standard decoder (handles trailing content after the JSON object).
	dec := json.NewDecoder(strings.NewReader(clean))
	if err := dec.Decode(v); err == nil {
		return nil
	}

	// Second attempt: repair stray unescaped quotes, then decode.
	repaired := repairJSONStrings(clean)
	dec2 := json.NewDecoder(strings.NewReader(repaired))
	return dec2.Decode(v)
}
