package prints

import (
	"fmt"
	"io"
)

func Fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}

func Fprintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}
