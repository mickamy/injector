package cli

import (
	"fmt"
	"io"
)

func fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}
