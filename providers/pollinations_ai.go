package providers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

const pollinationsAIAPIURL = "https://image.pollinations.ai/prompt/"

// PollinationsAIProvider implements the ImageProvider for Pollinations.ai.
type PollinationsAIProvider struct {
	APIKey string
	Client *http.Client
}

var pollinationsAIModels = []ModelCapabilities{
	{Name: "flux", SupportedParams: []string{"seed"}, MaxWidth: 1024, MaxHeight: 1024},
	{Name: "kontext", SupportedParams: []string{"seed", "image"}, MaxWidth: 1024, MaxHeight: 1024},
}

// NewPollinationsAIProvider creates a new Pollinations.ai client.
func NewPollinationsAIProvider(apiKey string) *PollinationsAIProvider {
	return &PollinationsAIProvider{
		APIKey: apiKey,
		Client: &http.Client{},
	}
}

// GetName returns the name of the provider.
func (p *PollinationsAIProvider) GetName() string {
	return "Pollinations_ai"
}

// RequiresImageURL returns true as Pollinations.ai requires an image URL.
func (p *PollinationsAIProvider) RequiresImageURL() bool {
	return true
}

// GetModels returns the list of models and their capabilities for Pollinations.ai.
func (p *PollinationsAIProvider) GetModels() []ModelCapabilities {
	return pollinationsAIModels
}

// Generate sends a request to the Pollinations.ai API.
func (p *PollinationsAIProvider) Generate(input GenerationInput) (*GenerationOutput, error) {
	// The prompt is always part of the path, and needs to be path-escaped.
	encodedPrompt := url.PathEscape(input.Prompt)
	fullURL := pollinationsAIAPIURL + encodedPrompt

	params := url.Values{}

	// This provider requires an image URL. The main handler is now responsible
	// for uploading the image and providing the URL in input.ImageURL.
	if input.ImageURL == "" && input.Model == "kontext" { // Kontext model requires an image
		return nil, fmt.Errorf("Pollinations_ai: model '%s' requires an image URL", input.Model)
	}

	// For img2img, add the final image URL as a query parameter.
	if input.ImageURL != "" {
		params.Add("image", input.ImageURL)
	}

	// Add remaining query parameters
	params.Add("model", input.Model)
	params.Add("width", fmt.Sprintf("%d", input.Width))
	params.Add("height", fmt.Sprintf("%d", input.Height))
	if input.Seed != 0 {
		params.Add("seed", fmt.Sprintf("%d", input.Seed))
	}
	params.Add("nologo", "true")

	// Append encoded parameters
	if query := params.Encode(); query != "" {
		fullURL += "?" + query
	}

	log.Printf("Calling provider '%s' with model '%s'", p.GetName(), input.Model)
	log.Printf("Request URL: %s", fullURL)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("Pollinations_ai: failed to create request: %w", err)
	}

	if p.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}

	var resp *http.Response
	const maxRetries = 4 // 1 initial attempt + 3 retries
	const retryInterval = 3 * time.Second

	for i := 0; i < maxRetries; i++ {
		resp, err = p.Client.Do(req)
		if err != nil {
			log.Printf("Error from provider '%s' on attempt %d/%d: %v", p.GetName(), i+1, maxRetries, err)
			if i < maxRetries-1 {
				log.Printf("Retrying in %v...", retryInterval)
				time.Sleep(retryInterval)
				continue
			}
			return nil, fmt.Errorf("Pollinations_ai: failed to call external API after %d attempts: %w", maxRetries, err)
		}

		if resp.StatusCode == http.StatusOK {
			break // Success
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close() // Must close body to reuse connection.
		err = fmt.Errorf("API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
		log.Printf("Error from provider '%s' on attempt %d/%d: %v", p.GetName(), i+1, maxRetries, err)

		if i < maxRetries-1 {
			log.Printf("Retrying in %v...", retryInterval)
			time.Sleep(retryInterval)
			continue
		}

		return nil, fmt.Errorf("Pollinations_ai: giving up after %d attempts: %w", maxRetries, err)
	}
	defer resp.Body.Close()

	// The response is the raw image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Pollinations_ai: failed to read image data: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	format := "png" // Default to png
	if contentType == "image/jpeg" {
		format = "jpeg"
	}

	return &GenerationOutput{
		ImageBytes: imageData,
		Format:     format,
	}, nil
}

// GetPollinationsAIAPIKey retrieves the API key from environment variables.
func GetPollinationsAIAPIKey() string {
	return os.Getenv("POLLINATIONS_AI_API_KEY")
}
