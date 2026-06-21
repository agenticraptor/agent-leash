package guard

import "strings"

// segment is one command in a (possibly chained) command line, split on shell
// operators. Args holds the shell-tokenized words of the command; Redirects
// holds any files the command writes to via `>` or `>>`.
type segment struct {
	Args      []string
	Redirects []string
}

// splitSegments tokenizes a command line with minimal shell awareness — enough
// to separate chained commands and respect quoting — and returns one segment
// per command. It is intentionally conservative: it does not execute or fully
// parse the shell grammar, it just isolates command words, operators, and
// redirect targets so each command can be inspected.
func splitSegments(line string) []segment {
	var (
		segs   []segment
		cur    segment
		token  []rune
		hasTok bool
		quote  byte // active quote byte, or 0
		redir  bool // the next finished token is a redirect target
	)
	runes := []rune(line)

	flushToken := func() {
		if !hasTok {
			return
		}
		t := string(token)
		if redir {
			cur.Redirects = append(cur.Redirects, t)
			redir = false
		} else {
			cur.Args = append(cur.Args, t)
		}
		token = token[:0]
		hasTok = false
	}
	flushSegment := func() {
		flushToken()
		if len(cur.Args) > 0 || len(cur.Redirects) > 0 {
			segs = append(segs, cur)
		}
		cur = segment{}
	}

	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if quote != 0 { // inside quotes everything is literal
			if byte(c) == quote {
				quote = 0
			} else {
				token = append(token, c)
				hasTok = true
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = byte(c)
			hasTok = true // an empty quoted string is still a token
		case ' ', '\t':
			flushToken()
		case '>':
			flushToken()
			if i+1 < len(runes) && runes[i+1] == '>' {
				i++
			}
			redir = true
		case '<':
			flushToken()
		case ';', '\n':
			flushSegment()
		case '&', '|':
			flushSegment()
			if i+1 < len(runes) && runes[i+1] == c {
				i++ // consume the second char of && or ||
			}
		default:
			token = append(token, c)
			hasTok = true
		}
	}
	flushSegment()
	return segs
}

// normalize collapses runs of whitespace to single spaces and trims the line,
// so patterns like "rm -rf /*" match regardless of original spacing.
func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// globMatch reports whether s matches a wildcard pattern using `*` (any run of
// characters, including none) and `?` (exactly one character), anchored to the
// whole string. Unlike filepath.Match, `*` spans `/` and spaces — which is what
// we want when matching command lines rather than file paths.
func globMatch(pattern, s string) bool {
	sIdx, pIdx := 0, 0
	starIdx, sTmpIdx := -1, -1
	for sIdx < len(s) {
		switch {
		case pIdx < len(pattern) && (pattern[pIdx] == '?' || pattern[pIdx] == s[sIdx]):
			sIdx++
			pIdx++
		case pIdx < len(pattern) && pattern[pIdx] == '*':
			starIdx = pIdx
			sTmpIdx = sIdx
			pIdx++
		case starIdx == -1:
			return false
		default:
			pIdx = starIdx + 1
			sTmpIdx++
			sIdx = sTmpIdx
		}
	}
	for pIdx < len(pattern) && pattern[pIdx] == '*' {
		pIdx++
	}
	return pIdx == len(pattern)
}
