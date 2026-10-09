package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
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
