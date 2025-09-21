package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	_ "image/png" // Keep for decoding pngs
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nfnt/resize"
	_ "golang.org/x/image/webp"
)

// APIPayload matches the structure of the JSON data for the Dreamifly API.
type APIPayload struct {
	Prompt    string   `json:"prompt"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	Steps     int      `json:"steps"`
	Seed      int64    `json:"seed"`
	BatchSize int      `json:"batch_size"`
	Model     string   `json:"model"`
	Images    []string `json:"images"`
	Denoise   float64  `json:"denoise"`
}

// OptimizePromptPayload matches the structure for the prompt optimization API.
type OptimizePromptPayload struct {
	Prompt string `json:"prompt"`
}

// OptimizePromptResponse matches the structure of the successful response.
type OptimizePromptResponse struct {
	Success         bool   `json:"success"`
	OriginalPrompt  string `json:"originalPrompt"`
	OptimizedPrompt string `json:"optimizedPrompt"`
}

func main() {
	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Serve the index page
	http.HandleFunc("/", serveIndex)

	// Handle the API requests
	http.HandleFunc("/api/generate", handleGenerate)
	http.HandleFunc("/api/optimize-prompt", handleOptimizePrompt)

	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Could not parse template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a new request for /api/generate")
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s", r.Method)
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Ensure images directory exists
	if err := os.MkdirAll("images", 0755); err != nil {
		log.Printf("Error creating images directory: %v", err)
	}

	// Parse multipart form, as we might have an image
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB
		log.Printf("Error parsing multipart form: %v", err)
		http.Error(w, "Could not parse multipart form", http.StatusBadRequest)
		return
	}

	var images []string
	var logMessage string

	// Get file from form, but don't error if it's missing
	file, handler, err := r.FormFile("image")
	if err != nil && err != http.ErrMissingFile {
		log.Printf("Error retrieving image from form: %v", err)
		http.Error(w, "Could not retrieve image from form", http.StatusBadRequest)
		return
	}

	// If a file was uploaded, process it
	if err == nil {
		defer file.Close()

		imgBytes, err := io.ReadAll(file)
		if err != nil {
			log.Printf("Error reading image file: %v", err)
			http.Error(w, "Could not read image file", http.StatusInternalServerError)
			return
		}

		processedImgBytes, err := processImage(imgBytes)
		if err != nil {
			log.Printf("Error processing image: %v", err)
			http.Error(w, "Could not process image", http.StatusInternalServerError)
			return
		}

		encodedImage := base64.StdEncoding.EncodeToString(processedImgBytes)
		images = []string{encodedImage}

		// Store info for the log message
		logMessage = fmt.Sprintf("Image: %s (original: %d bytes, processed: %d bytes)",
			handler.Filename, len(imgBytes), len(processedImgBytes))
	}

	// Parse other form fields
	prompt := r.FormValue("prompt")
	model := r.FormValue("model")
	steps, _ := strconv.Atoi(r.FormValue("steps"))
	seed, _ := strconv.ParseInt(r.FormValue("seed"), 10, 64)
	width, err := strconv.Atoi(r.FormValue("width"))
	if err != nil || width == 0 {
		width = 1920
	}
	height, err := strconv.Atoi(r.FormValue("height"))
	if err != nil || height == 0 {
		height = 1920
	}

	// Validate width and height
	if width < 64 {
		width = 64
	} else if width > 1920 {
		width = 1920
	}
	if height < 64 {
		height = 64
	} else if height > 1920 {
		height = 1920
	}

	if logMessage != "" {
		log.Printf("Received generation request. Prompt: '%s', Model: '%s', Steps: %d, Seed: %d, Width: %d, Height: %d, %s",
			prompt, model, steps, seed, width, height, logMessage)
	} else {
		log.Printf("Received generation request. Prompt: '%s', Model: '%s', Steps: %d, Seed: %d, Width: %d, Height: %d",
			prompt, model, steps, seed, width, height)
	}

	// Create payload for the external API
	payload := APIPayload{
		Prompt:    prompt,
		Width:     width,
		Height:    height,
		Steps:     steps,
		Seed:      seed,
		BatchSize: 1,
		Model:     model,
		Images:    images,
		Denoise:   0.7,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling API payload: %v", err)
		http.Error(w, "Could not marshal API payload", http.StatusInternalServerError)
		return
	}
	log.Println("Calling external API...")

	// Call the external API
	req, err := http.NewRequest("POST", "https://dreamifly.com/api/generate", bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Printf("Error creating request to external API: %v", err)
		http.Error(w, "Could not create request to external API", http.StatusInternalServerError)
		return
	}

	// Set headers as per the curl command
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://dreamifly.com/zh")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("sec-ch-ua", `"Chromium";v="140", "Not=A?Brand";v="24", "Google Chrome";v="140"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error calling external API: %v", err)
		http.Error(w, "Failed to call external API", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	log.Printf("External API responded with status code: %d, Content-Type: %s", resp.StatusCode, resp.Header.Get("Content-Type"))

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("External API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
		http.Error(w, "External API returned an error", resp.StatusCode)
		return
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		http.Error(w, "Could not read image data", http.StatusInternalServerError)
		return
	}

	// Try to parse response as JSON with base64 image data
	type ImageResponse struct {
		ImageURL string `json:"imageUrl"`
	}
	var imageResp ImageResponse
	var imageData []byte
	var contentType string

	if err := json.Unmarshal(respBody, &imageResp); err == nil && imageResp.ImageURL != "" {
		// It's a JSON response with an imageUrl, process it as a data URL
		dataURL := imageResp.ImageURL
		if strings.HasPrefix(dataURL, "data:image/") {
			commaIndex := strings.Index(dataURL, ",")
			if commaIndex != -1 {
				// Extract mime type from prefix, e.g., "data:image/png;base64"
				prefix := dataURL[:commaIndex]
				parts := strings.Split(prefix, ";")
				if len(parts) > 0 {
					mimeParts := strings.Split(parts[0], ":")
					if len(mimeParts) > 1 {
						contentType = mimeParts[1]
					}
				}

				// Fallback if content type parsing fails
				if contentType == "" {
					log.Println("Could not determine content type from data URL, defaulting to image/png")
					contentType = "image/png"
				}

				base64Data := dataURL[commaIndex+1:]
				imageData, err = base64.StdEncoding.DecodeString(base64Data)
				if err != nil {
					log.Printf("Error decoding base64 image data: %v", err)
					http.Error(w, "Could not decode image data", http.StatusInternalServerError)
					return
				}
			} else {
				log.Printf("Invalid data URL format: missing comma")
				http.Error(w, "Invalid image data format", http.StatusInternalServerError)
				return
			}
		} else {
			log.Printf("Unexpected image URL format: does not start with 'data:image/'")
			http.Error(w, "Invalid image data format", http.StatusInternalServerError)
			return
		}
	} else {
		// If not JSON or doesn't contain imageUrl, treat the raw response as image data
		imageData = respBody
		contentType = http.DetectContentType(imageData)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102150405000")
	filename := fmt.Sprintf("images/%s.png", timestamp)

	// Save image to file
	if err := os.WriteFile(filename, imageData, 0644); err != nil {
		log.Printf("Error saving image to file: %v", err)
	} else {
		log.Printf("Image saved to %s", filename)
	}

	// Stream the image data back to the client
	w.Header().Set("Content-Type", contentType)
	w.Write(imageData)
	log.Println("Successfully streamed image response to client.")
}

func handleOptimizePrompt(w http.ResponseWriter, r *http.Request) {
	log.Println("Received a new request for /api/optimize-prompt")
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s", r.Method)
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode the incoming JSON payload
	var payload OptimizePromptPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Optimizing prompt: '%s'", payload.Prompt)

	// Create payload for the external API
	apiPayloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling API payload: %v", err)
		http.Error(w, "Could not marshal API payload", http.StatusInternalServerError)
		return
	}

	// Call the external API
	req, err := http.NewRequest("POST", "https://dreamifly.com/api/optimize-prompt", bytes.NewBuffer(apiPayloadBytes))
	if err != nil {
		log.Printf("Error creating request to external API: %v", err)
		http.Error(w, "Could not create request to external API", http.StatusInternalServerError)
		return
	}

	// Set headers to mimic the provided curl command
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "https://dreamifly.com")
	req.Header.Set("Referer", "https://dreamifly.com/zh")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error calling external API: %v", err)
		http.Error(w, "Failed to call external API", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	log.Printf("External API responded with status code: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("External API returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
		http.Error(w, "External API returned an error", resp.StatusCode)
		return
	}

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		http.Error(w, "Could not read response from external API", http.StatusInternalServerError)
		return
	}

	// Forward the response to the client
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBody)
	log.Println("Successfully forwarded API response to client.")
}

// processImage checks if an image is larger than 1920x1920 and resizes it if necessary.
// It also converts PNG images to JPEG.
func processImage(imgBytes []byte) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	log.Printf("Decoded image format: %s", format)

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	needsResize := width > 1920 || height > 1920
	needsConversion := format == "png" || format == "webp"

	if !needsResize && !needsConversion {
		log.Println("Image is within size limits and does not need conversion, no processing needed.")
		return imgBytes, nil
	}

	var processedImage image.Image
	if needsResize {
		log.Printf("Image original size: %dx%d. Resizing to max 1920.", width, height)
		processedImage = resize.Thumbnail(1920, 1920, img, resize.Lanczos3)
	} else {
		processedImage = img
	}

	var buf bytes.Buffer
	// Re-encode the image to JPEG with a quality setting.
	if err := jpeg.Encode(&buf, processedImage, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("failed to encode image to jpeg: %w", err)
	}

	if needsResize {
		log.Printf("Image resized to: %dx%d", processedImage.Bounds().Dx(), processedImage.Bounds().Dy())
	}
	if needsConversion {
		log.Printf("Converted %s image to JPEG.", format)
	}

	return buf.Bytes(), nil
}
