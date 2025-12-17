package scan

import (
	"encoding/csv"
	"fmt"
	"go/token"
	"strings"
)

func position(fset *token.FileSet, pos token.Pos) string {
	if fset == nil {
		return ""
	}
	p := fset.Position(pos)
	if !p.IsValid() {
		return ""
	}
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

func joinLines(lines []string) string {
	if len(lines) == 1 {
		return lines[0]
	}
	var b strings.Builder
	for i, s := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s)
	}
	return b.String()
}

func splitDirectives(raw string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(raw))
	r.Comma = ','
	r.TrimLeadingSpace = true

	records, err := r.Read()
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(records))
	for _, s := range records {
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func cutKV(s string) (key string, val string, ok bool) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(s[:i])
	val = strings.TrimSpace(s[i+1:])
	if key == "" {
		return "", "", false
	}
	return key, val, true
}
