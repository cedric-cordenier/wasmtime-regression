package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
)

var re = regexp.MustCompile(`^module-[0-9]+$`)

func main() {
	sum := sha256.Sum256([]byte(time.Now().String()))
	n := new(big.Int).SetBytes(sum[:])

	var sb strings.Builder
	if err := template.Must(template.New("t").Parse("{{.}}")).Execute(&sb, n.String()); err != nil {
		panic(err)
	}

	parts := strings.Fields(sb.String())
	sort.Strings(parts)

	body, err := json.Marshal(map[string]any{
		"parts":  parts,
		"match":  re.MatchString("module-1"),
		"status": http.StatusOK,
	})
	if err != nil {
		panic(err)
	}

	zw := gzip.NewWriter(os.Stdout)
	if _, err := zw.Write(body); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}

	fmt.Fprintln(os.Stderr, "ok")
}
