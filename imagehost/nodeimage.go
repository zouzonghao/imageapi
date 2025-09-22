package imagehost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

const (
	uploadAPIURL = "https://api.nodeimage.com/api/upload"
	deleteAPIURL = "https://api.nodeimage.com/api/v1/delete/"
)

// NodeImageClient handles communication with the NodeImage API.
type NodeImageClient struct {
	APIKey string
	Client *http.Client
}

// NewNodeImageClient creates a new NodeImage client.
func NewNodeImageClient(apiKey string) *NodeImageClient {
	return &NodeImageClient{
		APIKey: apiKey,
		Client: &http.Client{},
	}
}

// UploadResponse matches the structure of the successful upload response.
type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	ImageID string `json:"image_id"`
	Links   struct {
		Direct string `json:"direct"`
	} `json:"links"`
}

// DeleteResponse matches the structure of the successful delete response.
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// UploadImage uploads an image and returns the direct URL and image ID.
func (c *NodeImageClient) UploadImage(imageBytes []byte, filename string) (*UploadResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	_, err = io.Copy(part, bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to copy image bytes to form: %w", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", uploadAPIURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nodeimage API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	var uploadResp UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, fmt.Errorf("failed to decode upload response: %w", err)
	}

	if !uploadResp.Success {
		return nil, fmt.Errorf("nodeimage API reported an error: %s", uploadResp.Message)
	}

	return &uploadResp, nil
}

// DeleteImage deletes an image by its ID.
func (c *NodeImageClient) DeleteImage(imageID string) error {
	deleteURL := deleteAPIURL + imageID
	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	req.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("nodeimage API returned non-200 status for delete: %d, body: %s", resp.StatusCode, string(body))
	}

	var deleteResp DeleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&deleteResp); err != nil {
		return fmt.Errorf("failed to decode delete response: %w", err)
	}

	if !deleteResp.Success {
		return fmt.Errorf("nodeimage API reported an error on delete: %s", deleteResp.Message)
	}

	return nil
}
