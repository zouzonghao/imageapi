package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	modelScopeAPIURL   = "https://api-inference.modelscope.cn/v1/images/generations"
	modelScopeTaskURL  = "https://api-inference.modelscope.cn/v1/tasks/"
	maxPollingAttempts = 90 // 5 minutes timeout (60 attempts * 5 seconds)
	pollingInterval    = 5 * time.Second
)

// ModelScopeProvider implements the ImageProvider for ModelScope.
type ModelScopeProvider struct {
	APIKey string
	Client *http.Client
}

var modelScopeModels = []ModelCapabilities{
	{Name: "Qwen/Qwen-Image", SupportedParams: []string{"seed"}, MaxWidth: 2048, MaxHeight: 2048},
	{Name: "Qwen/Qwen-Image-Edit", SupportedParams: []string{"seed", "image"}, MaxWidth: 2048, MaxHeight: 2048},
}

// NewModelScopeProvider creates a new ModelScope client.
func NewModelScopeProvider(apiKey string) *ModelScopeProvider {
	return &ModelScopeProvider{
		APIKey: apiKey,
		Client: &http.Client{},
	}
}

// GetName returns the name of the provider.
func (p *ModelScopeProvider) GetName() string {
	return "Modelscope"
}

// RequiresImageURL returns true as ModelScope requires an image URL.
func (p *ModelScopeProvider) RequiresImageURL() bool {
	return true
}

// GetModels returns the list of models and their capabilities for ModelScope.
func (p *ModelScopeProvider) GetModels() []ModelCapabilities {
	return modelScopeModels
}

type modelScopeAPIPayload struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	ImageURL string `json:"image_url,omitempty"`
	Size     string `json:"size"`
}

type modelScopeAsyncResponse struct {
	TaskID string `json:"task_id"`
}

type modelScopeErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type modelScopeTaskResponse struct {
	TaskStatus   string                `json:"task_status"`
	OutputImages []string              `json:"output_images"`
	Errors       modelScopeErrorDetail `json:"errors,omitempty"`
}

// Generate sends a request to the ModelScope API and polls for the result.
func (p *ModelScopeProvider) Generate(input GenerationInput) (*GenerationOutput, error) {
	size := fmt.Sprintf("%dx%d", input.Width, input.Height)
	payload := modelScopeAPIPayload{
		Model:    input.Model,
		Prompt:   input.Prompt,
		ImageURL: input.ImageURL, // Will be empty for text-to-image
		Size:     size,
	}

	logPayloadBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		log.Printf("Error marshalling log payload: %v", err)
	} else {
		log.Printf("Calling provider '%s' with model '%s'", p.GetName(), input.Model)
		log.Printf("Request payload: \n%s", string(logPayloadBytes))
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("Modelscope: failed to marshal payload: %w", err)
	}

	// 1. Initiate the generation task
	req, err := http.NewRequest("POST", modelScopeAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("Modelscope: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("X-ModelScope-Async-Mode", "true")

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Modelscope: failed to call generation API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Modelscope: generation API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	var asyncResp modelScopeAsyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&asyncResp); err != nil {
		return nil, fmt.Errorf("Modelscope: failed to decode async response: %w", err)
	}

	if asyncResp.TaskID == "" {
		return nil, fmt.Errorf("Modelscope: did not receive a task ID")
	}

	log.Printf("Modelscope: task submitted successfully, task_id: %s", asyncResp.TaskID)

	// 2. Poll for the result
	taskURL := modelScopeTaskURL + asyncResp.TaskID
	for i := 0; i < maxPollingAttempts; i++ {
		time.Sleep(pollingInterval)

		pollReq, err := http.NewRequest("GET", taskURL, nil)
		if err != nil {
			return nil, fmt.Errorf("Modelscope: failed to create polling request: %w", err)
		}
		pollReq.Header.Set("Authorization", "Bearer "+p.APIKey)
		pollReq.Header.Set("X-ModelScope-Task-Type", "image_generation")

		pollResp, err := p.Client.Do(pollReq)
		if err != nil {
			return nil, fmt.Errorf("Modelscope: failed to execute polling request: %w", err)
		}
		defer pollResp.Body.Close()

		if pollResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(pollResp.Body)
			// Log the error but continue polling, as the task might still be processing or temporarily unavailable
			log.Printf("Modelscope: polling returned non-200 status: %d, body: %s", pollResp.StatusCode, string(body))
			continue
		}

		bodyBytes, err := io.ReadAll(pollResp.Body)
		if err != nil {
			return nil, fmt.Errorf("Modelscope: failed to read polling response body: %w", err)
		}

		var taskResp modelScopeTaskResponse
		if err := json.Unmarshal(bodyBytes, &taskResp); err != nil {
			// Return the raw body if JSON decoding fails
			return nil, fmt.Errorf("Modelscope: failed to decode task response: %w, body: %s", err, string(bodyBytes))
		}

		switch taskResp.TaskStatus {
		case "SUCCEED":
			if len(taskResp.OutputImages) > 0 {
				imageURL := taskResp.OutputImages[0]
				imageData, _, err := DownloadFile(imageURL)
				if err != nil {
					return nil, fmt.Errorf("Modelscope: failed to download generated image: %w", err)
				}
				return &GenerationOutput{
					ImageBytes: imageData,
				}, nil
			}
			return nil, fmt.Errorf("Modelscope: task succeeded but no image URL was returned")
		case "FAILED", "CANCELED":
			errMsg := "Modelscope: task failed or was canceled"
			if taskResp.Errors.Message != "" {
				errMsg = fmt.Sprintf("%s. Reason: %s", errMsg, taskResp.Errors.Message)
			}
			// Also include the full body for debugging
			return nil, fmt.Errorf("%s. Full Response: %s", errMsg, string(bodyBytes))
		}
		// Otherwise, continue polling
	}

	return nil, fmt.Errorf("Modelscope: polling timed out after %d attempts", maxPollingAttempts)
}

// GetModelScopeAPIKey retrieves the API key from environment variables.
func GetModelScopeAPIKey() string {
	return os.Getenv("MODELSCOPE_API_KEY")
}
