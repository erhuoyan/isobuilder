package httpapi

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// parseIntQuery reads an integer query parameter.
func parseIntQuery(r *http.Request, name string) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, fmt.Errorf("missing %s", name)
	}
	return strconv.ParseInt(raw, 10, 64)
}

// normalizeScriptNewlines converts CRLF and lone CR line endings to LF.
// Text pasted from Windows editors or some terminals carries \r\n or bare \r;
// browsers and kickstart's base64 injection expect \n. Without this, a script
// renders as one long line in the preview drawer and may misbehave at install
// time (bash tolerates \r\n but not bare \r in shebangs and here-docs).
func normalizeScriptNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// md5File computes the MD5 digest of a file (装机侧校验习惯用 MD5).
func md5File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// sha256File hashes a file and returns the digest and size.
func sha256File(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// humanBytes renders a byte count the way an operator reads it.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	value := float64(n)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.2f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.2f PB", value/unit)
}

// nowRFC3339 is the timestamp format used in API responses.
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }
