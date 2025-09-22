package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

const falAIAPIURL = "https://fal.run/fal-ai/bytedance/seedream/v4/edit"

// FalAIProvider implements the ImageProvider for Fal.ai.
type FalAIProvider struct {
	APIKey string
	Client *http.Client
}

var falAIModels = []ModelCapabilities{
	{Name: "bytedance/seedream/v4/edit", SupportedParams: []string{"seed", "image"}, MaxWidth: 4096, MaxHeight: 4096},
}

// NewFalAIProvider creates a new Fal.ai client.
func NewFalAIProvider(apiKey string) *FalAIProvider {
	return &FalAIProvider{
		APIKey: apiKey,
		Client: &http.Client{},
	}
}

// GetName returns the name of the provider.
func (p *FalAIProvider) GetName() string {
	return "fal_ai"
}

// RequiresImageURL returns true as Fal.ai requires an image URL.
func (p *FalAIProvider) RequiresImageURL() bool {
	return true
}

// GetModels returns the list of models and their capabilities for Fal.ai.
func (p *FalAIProvider) GetModels() []ModelCapabilities {
	return falAIModels
}

type falAIAPIPayload struct {
	Prompt    string   `json:"prompt"`
	ImageURLs []string `json:"image_urls"`
	ImageSize struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"image_size"`
	EnableSafetyChecker bool `json:"enable_safety_checker"`
}

type falAIAPIResponse struct {
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
}

// Generate sends a request to the Fal.ai API.
// Note: Fal.ai requires an image URL, so the controller logic
// will need to ensure input.ImageURL is populated.
func (p *FalAIProvider) Generate(input GenerationInput) (*GenerationOutput, error) {
	// This provider requires an image URL. The main handler should have uploaded
	// the image if bytes were provided.
	if input.ImageURL == "" {
		return nil, fmt.Errorf("fal_ai: image URL is required")
	}

	payload := falAIAPIPayload{
		Prompt:              input.Prompt,
		ImageURLs:           []string{input.ImageURL},
		EnableSafetyChecker: false,
	}
	payload.ImageSize.Width = input.Width
	payload.ImageSize.Height = input.Height

	logPayloadBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Printf("Error marshalling log payload: %v", err)
	} else {
		log.Printf("Calling provider '%s' with model '%s'", p.GetName(), input.Model)
		log.Printf("Request payload: \n%s", string(logPayloadBytes))
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("fal_ai: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", falAIAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("fal_ai: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Key "+p.APIKey)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fal_ai: failed to call external API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fal_ai: API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	var apiResp falAIAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("fal_ai: failed to decode response: %w", err)
	}

	if len(apiResp.Images) == 0 {
		return nil, fmt.Errorf("fal_ai: no images returned in response")
	}

	// The response from Fal.ai is a URL. Download the image bytes.
	imageURL := apiResp.Images[0].URL
	imageData, _, err := DownloadFile(imageURL)
	if err != nil {
		return nil, fmt.Errorf("fal_ai: failed to download generated image: %w", err)
	}

	return &GenerationOutput{
		ImageBytes: imageData,
	}, nil
}

// GetFalAIAPIKey retrieves the API key from environment variables.
func GetFalAIAPIKey() string {
	return os.Getenv("FAL_API_KEY")
}
