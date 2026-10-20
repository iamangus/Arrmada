package store

import (
	"strconv"
	"strings"
)

// bind only changes parameter markers, never SQL syntax or values. PostgreSQL
// literals, identifiers, dollar quotes and comments can contain question marks.
// An actual PostgreSQL question-mark operator is written ?? in a query sent
// through this boundary (backend-specific queries may still use $n parameters).
func (d Dialect) bind(query string) string {
	if d != Postgres || !strings.Contains(query, "?") {
		return query
	}
	var out strings.Builder
	out.Grow(len(query))
	param := 0
	for i := 0; i < len(query); {
		start := i
		switch {
		case query[i] == '\'' || query[i] == '"':
			quote := query[i]
			// Backslash escaping applies to E'...' only, not standard strings.
			escape := quote == '\'' && i > 0 && (query[i-1] == 'e' || query[i-1] == 'E') &&
				(i < 2 || !sqlIdentByte(query[i-2]))
			i++
			for i < len(query) {
				if escape && query[i] == '\\' {
					i++
					if i < len(query) {
						i++
					}
					continue
				}
				if query[i] == quote {
					i++
					if i < len(query) && query[i] == quote {
						i++
						continue
					}
					break
				}
				i++
			}
		case strings.HasPrefix(query[i:], "--"):
			if n := strings.IndexAny(query[i:], "\r\n"); n >= 0 {
				i += n + 1
			} else {
				i = len(query)
			}
		case strings.HasPrefix(query[i:], "/*"):
			i += 2
			depth := 1
			for i < len(query) && depth > 0 {
				switch {
				case strings.HasPrefix(query[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(query[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
		case query[i] == '$':
			// A dollar-quote opener must not continue an identifier. $1 is
			// already a native PostgreSQL parameter, not a dollar quote.
			end := i + 1
			if i == 0 || !sqlIdentByte(query[i-1]) {
				for end < len(query) && sqlTagByte(query[end], end == i+1) {
					end++
				}
			}
			if end < len(query) && query[end] == '$' && (i == 0 || !sqlIdentByte(query[i-1])) {
				delim := query[i : end+1]
				i = end + 1
				if n := strings.Index(query[i:], delim); n >= 0 {
					i += n + len(delim)
				} else {
					i = len(query)
				}
			} else {
				i++
			}
		case query[i] == '?':
			if i+1 < len(query) && query[i+1] == '?' {
				out.WriteByte('?')
				i += 2
			} else {
				param++
				out.WriteByte('$')
				out.WriteString(strconv.Itoa(param))
				i++
			}
			continue
		default:
			i++
		}
		out.WriteString(query[start:i])
	}
	return out.String()
}

func sqlTagByte(c byte, first bool) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 128 ||
		!first && c >= '0' && c <= '9'
}

func sqlIdentByte(c byte) bool { return sqlTagByte(c, false) || c == '$' }
