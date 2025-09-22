package providers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const (
	dreamiflyAPIURL               = "https://dreamifly.com/api/generate"
	dreamiflyOptimizePromptAPIURL = "https://dreamifly.com/api/optimize-prompt"
)

// DreamiflyProvider implements the ImageProvider for Dreamifly.
type DreamiflyProvider struct {
	Client *http.Client
}

var dreamiflyModels = []ModelCapabilities{
	{Name: "Flux-Kontext", SupportedParams: []string{"steps", "seed", "image"}, MaxWidth: 1920, MaxHeight: 1920},
	{Name: "Qwen-Image-Edit", SupportedParams: []string{"steps", "seed", "image"}, MaxWidth: 1920, MaxHeight: 1920},
	{Name: "Wai-SDXL-V150", SupportedParams: []string{"steps", "seed"}, MaxWidth: 1920, MaxHeight: 1920},
	{Name: "Flux-Krea", SupportedParams: []string{"steps", "seed"}, MaxWidth: 1920, MaxHeight: 1920},
	{Name: "HiDream-full-fp8", SupportedParams: []string{"steps", "seed"}, MaxWidth: 1920, MaxHeight: 1920},
	{Name: "Qwen-Image", SupportedParams: []string{"steps", "seed"}, MaxWidth: 1920, MaxHeight: 1920},
}

// NewDreamiflyProvider creates a new Dreamifly client.
func NewDreamiflyProvider() *DreamiflyProvider {
	return &DreamiflyProvider{
		Client: &http.Client{},
	}
}

// GetName returns the name of the provider.
func (p *DreamiflyProvider) GetName() string {
	return "Dreamifly"
}

// RequiresImageURL returns false as Dreamifly accepts image bytes directly.
func (p *DreamiflyProvider) RequiresImageURL() bool {
	return false
}

// GetModels returns the list of models and their capabilities for Dreamifly.
func (p *DreamiflyProvider) GetModels() []ModelCapabilities {
	return dreamiflyModels
}

// APIPayload matches the structure for the Dreamifly API.
type dreamiflyAPIPayload struct {
	Prompt    string   `json:"prompt"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Steps     int      `json:"steps,omitempty"`
	Seed      int64    `json:"seed"`
	BatchSize int      `json:"batch_size"`
	Model     string   `json:"model"`
	Images    []string `json:"images"`
	Denoise   float64  `json:"denoise"` // Defaulting to 0.7
}

// ImageResponse matches the JSON response with base64 image data.
type dreamiflyImageResponse struct {
	ImageURL string `json:"imageUrl"`
}

// OptimizePrompt sends a request to the Dreamifly API to optimize a prompt.
func (p *DreamiflyProvider) OptimizePrompt(prompt string) (string, error) {
	payload := struct {
		Prompt string `json:"prompt"`
	}{
		Prompt: prompt,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("dreamifly: failed to marshal optimize prompt payload: %w", err)
	}

	req, err := http.NewRequest("POST", dreamiflyOptimizePromptAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("dreamifly: failed to create optimize prompt request: %w", err)
	}

	// Set headers from API doc
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "https://dreamifly.com")
	req.Header.Set("Referer", "https://dreamifly.com/zh")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36")

	log.Printf("Calling Dreamifly prompt optimization for: \"%s\"", prompt)

	resp, err := p.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("dreamifly: failed to call optimize prompt API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("dreamifly: optimize prompt API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	var optimizeResp struct {
		Success         bool   `json:"success"`
		OriginalPrompt  string `json:"originalPrompt"`
		OptimizedPrompt string `json:"optimizedPrompt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&optimizeResp); err != nil {
		return "", fmt.Errorf("dreamifly: failed to decode optimize prompt response: %w", err)
	}

	if !optimizeResp.Success {
		return "", fmt.Errorf("dreamifly: optimize prompt API reported failure")
	}

	return optimizeResp.OptimizedPrompt, nil
}

// Generate sends a request to the Dreamifly API.
func (p *DreamiflyProvider) Generate(input GenerationInput) (*GenerationOutput, error) {
	var images []string
	if len(input.ImageBytes) > 0 {
		encodedImage := base64.StdEncoding.EncodeToString(input.ImageBytes)
		images = []string{encodedImage}
	}

	steps := input.Steps
	if steps == 0 {
		steps = 25 // A reasonable default if not provided
	}

	payload := dreamiflyAPIPayload{
		Prompt:    input.Prompt,
		Width:     input.Width,
		Height:    input.Height,
		Steps:     steps,
		Seed:      input.Seed,
		BatchSize: 1,
		Model:     input.Model,
		Images:    images,
		Denoise:   0.7,
	}

	// Create a copy of the payload for logging, but without the image data.
	logPayload := payload
	if len(logPayload.Images) > 0 {
		logPayload.Images = []string{"<image data omitted>"}
	}

	logPayloadBytes, err := json.MarshalIndent(logPayload, "", "  ") // Using MarshalIndent for readability
	if err != nil {
		log.Printf("Error marshalling log payload: %v", err)
	} else {
		log.Printf("Calling provider '%s' with model '%s'", p.GetName(), input.Model)
		log.Printf("Request payload: \n%s", string(logPayloadBytes))
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("dreamifly: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", dreamiflyAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("dreamifly: failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://dreamifly.com/zh")

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dreamifly: failed to call external API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dreamifly: API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("dreamifly: failed to read response body: %w", err)
	}

	var imageResp dreamiflyImageResponse
	var imageData []byte

	if err := json.Unmarshal(respBody, &imageResp); err == nil && imageResp.ImageURL != "" {
		// It's a JSON response with a data URL
		dataURL := imageResp.ImageURL
		if strings.HasPrefix(dataURL, "data:image/") {
			commaIndex := strings.Index(dataURL, ",")
			if commaIndex != -1 {
				base64Data := dataURL[commaIndex+1:]
				imageData, err = base64.StdEncoding.DecodeString(base64Data)
				if err != nil {
					return nil, fmt.Errorf("dreamifly: failed to decode base64 image data: %w", err)
				}
			} else {
				return nil, fmt.Errorf("dreamifly: invalid data URL format")
			}
		} else {
			return nil, fmt.Errorf("dreamifly: unexpected image URL format")
		}
	} else {
		// Treat the raw response as image data
		imageData = respBody
	}

	return &GenerationOutput{
		ImageBytes: imageData,
	}, nil
}
