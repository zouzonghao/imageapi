package providers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DownloadFile downloads a file from a URL and returns its content and content type.
func DownloadFile(url string) ([]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("bad status: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, contentType, nil
}

// ParseModelName splits a full model name string into its provider and model parts.
// The expected format is "provider/model_name".
func ParseModelName(fullModelName string) (string, string, error) {
	parts := strings.SplitN(fullModelName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid model format. Expected 'provider/model_name', got '%s'", fullModelName)
	}
	return parts[0], parts[1], nil
}
