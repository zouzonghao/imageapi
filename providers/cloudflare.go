package providers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"imageapi/config"
)

const (
	cloudflareAPIURLFormat = "https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s"
)

// CloudflareProvider implements the ImageProvider for Cloudflare.
type CloudflareProvider struct {
	Client    *http.Client
	AccountID string
	APIToken  string
}

var cloudflareModels = []ModelCapabilities{
	{Name: "@cf/black-forest-labs/flux-1-schnell", SupportedParams: []string{"steps"}, MaxWidth: 1024, MaxHeight: 1024, MinSteps: 4, MaxSteps: 8, DefaultSteps: 8},
	{Name: "@cf/stabilityai/stable-diffusion-xl-base-1.0", SupportedParams: []string{"width", "height"}, MaxWidth: 1024, MaxHeight: 1024},
}

// NewCloudflareProvider creates a new Cloudflare client if credentials are provided.
func NewCloudflareProvider() *CloudflareProvider {
	accountID := config.AppConfig.CloudflareCredentials.AccountID
	apiToken := config.AppConfig.CloudflareCredentials.APIToken

	if accountID == "" || apiToken == "" {
		return nil // Return nil if credentials are not set
	}

	return &CloudflareProvider{
		Client:    &http.Client{},
		AccountID: accountID,
		APIToken:  apiToken,
	}
}

// GetName returns the name of the provider.
func (p *CloudflareProvider) GetName() string {
	return "Cloudflare"
}

// RequiresImageURL returns false as Cloudflare does not support image-to-image in this model.
func (p *CloudflareProvider) RequiresImageURL() bool {
	return false
}

// GetModels returns the list of models and their capabilities for Cloudflare.
func (p *CloudflareProvider) GetModels() []ModelCapabilities {
	return cloudflareModels
}

// cloudflareAPIPayload matches the structure for the Cloudflare API.
type cloudflareAPIPayload struct {
	Prompt string `json:"prompt"`
	Steps  int    `json:"steps,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// cloudflareImageResponse matches the JSON response with base64 image data.
type cloudflareImageResponse struct {
	Result struct {
		Image string `json:"image"`
	} `json:"result"`
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

// Generate sends a request to the Cloudflare API.
func (p *CloudflareProvider) Generate(input GenerationInput) (*GenerationOutput, error) {
	payload := cloudflareAPIPayload{
		Prompt: input.Prompt,
	}

	var modelCaps ModelCapabilities
	found := false
	for _, m := range p.GetModels() {
		if m.Name == input.Model {
			modelCaps = m
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("cloudflare: model %s not found or not supported", input.Model)
	}

	// Helper to check if a parameter is supported by the current model
	isParamSupported := func(param string) bool {
		for _, supportedParam := range modelCaps.SupportedParams {
			if supportedParam == param {
				return true
			}
		}
		return false
	}

	if isParamSupported("steps") {
		payload.Steps = input.Steps
	}
	if isParamSupported("width") {
		payload.Width = input.Width
	}
	if isParamSupported("height") {
		payload.Height = input.Height
	}

	logPayloadBytes, _ := json.MarshalIndent(payload, "", "  ")
	log.Printf("Calling provider '%s' with model '%s'", p.GetName(), input.Model)
	log.Printf("Request payload: \n%s", string(logPayloadBytes))

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: failed to marshal payload: %w", err)
	}

	apiURL := fmt.Sprintf(cloudflareAPIURLFormat, p.AccountID, input.Model)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("cloudflare: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIToken)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: failed to call external API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cloudflare: API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	var imageData []byte

	if contentType == "image/png" {
		imageData, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("cloudflare: failed to read image response body: %w", err)
		}
	} else { // Assume JSON response for other cases
		var imageResp cloudflareImageResponse
		if err := json.NewDecoder(resp.Body).Decode(&imageResp); err != nil {
			return nil, fmt.Errorf("cloudflare: failed to decode json response body: %w", err)
		}

		if !imageResp.Success || len(imageResp.Errors) > 0 {
			if len(imageResp.Errors) > 0 {
				return nil, fmt.Errorf("cloudflare: API error: %s", imageResp.Errors[0].Message)
			}
			return nil, fmt.Errorf("cloudflare: API reported failure but returned no error details")
		}

		imageData, err = base64.StdEncoding.DecodeString(imageResp.Result.Image)
		if err != nil {
			return nil, fmt.Errorf("cloudflare: failed to decode base64 image data: %w", err)
		}
	}

	return &GenerationOutput{
		ImageBytes: imageData,
	}, nil
}
