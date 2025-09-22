package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/jpeg" // Import for decoding JPEGs
	_ "image/png"  // Import for decoding PNGs
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"dreamifly/imagehost"
	"dreamifly/providers"

	"github.com/chai2010/webp"
	"github.com/joho/godotenv"
	"github.com/nfnt/resize"
)

var (
	providerRegistry map[string]providers.ImageProvider
	imageHostClient  *imagehost.NodeImageClient
)

func main() {
	rand.Seed(time.Now().UnixNano())
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Could not load .env file")
	}

	// Initialize and register all providers
	initializeProviders()

	// Initialize the image host client
	nodeImageAPIKey := imagehost.GetNodeImageAPIKey()
	if nodeImageAPIKey == "" {
		log.Println("Warning: NODEIMAGE_API_KEY is not set. Image hosting will be disabled.")
	}
	imageHostClient = imagehost.NewNodeImageClient(nodeImageAPIKey)

	// Ensure images directory exists
	if err := os.MkdirAll("images", 0755); err != nil {
		log.Fatalf("Could not create images directory: %v", err)
	}

	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Serve the index page
	http.HandleFunc("/", serveIndex)

	// Handle the API requests
	http.HandleFunc("/api/generate", handleGenerate)
	http.HandleFunc("/api/models", handleGetModels)
	http.HandleFunc("/api/optimize-prompt", handleOptimizePrompt)

	log.Println("Starting server on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}

func initializeProviders() {
	providerRegistry = make(map[string]providers.ImageProvider)

	// Dreamifly (no API key needed)
	dreamifly := providers.NewDreamiflyProvider()
	providerRegistry[dreamifly.GetName()] = dreamifly

	// Fal.ai
	falAIAPIKey := providers.GetFalAIAPIKey()
	if falAIAPIKey != "" {
		falAI := providers.NewFalAIProvider(falAIAPIKey)
		providerRegistry[falAI.GetName()] = falAI
	} else {
		log.Println("Warning: FAL_API_KEY not set, Fal.ai provider disabled.")
	}

	// ModelScope
	modelScopeAPIKey := providers.GetModelScopeAPIKey()
	if modelScopeAPIKey != "" {
		modelScope := providers.NewModelScopeProvider(modelScopeAPIKey)
		providerRegistry[modelScope.GetName()] = modelScope
	} else {
		log.Println("Warning: MODELSCOPE_API_KEY not set, ModelScope provider disabled.")
	}

	// Pollinations.ai
	pollinationsAPIKey := providers.GetPollinationsAIAPIKey()
	// This provider can work without an API key, so we initialize it anyway.
	// The provider itself will handle whether to send the auth header.
	pollinations := providers.NewPollinationsAIProvider(pollinationsAPIKey)
	providerRegistry[pollinations.GetName()] = pollinations
	if pollinationsAPIKey == "" {
		log.Println("Info: POLLINATIONS_AI_API_KEY not set. Pollinations.ai provider will work without authentication.")
	}

	log.Printf("Initialized %d providers", len(providerRegistry))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/index.html")
}

// ModelDetail defines the structure for the model list API response.
type ModelDetail struct {
	Name            string   `json:"name"`
	SupportedParams []string `json:"supported_params"`
	MaxWidth        int      `json:"max_width"`
	MaxHeight       int      `json:"max_height"`
}

type ProviderInfo struct {
	Provider string        `json:"provider"`
	Models   []ModelDetail `json:"models"`
}

func handleGetModels(w http.ResponseWriter, r *http.Request) {
	var availableProviders []ProviderInfo

	// Iterate over the registered providers to dynamically build the response.
	for name, provider := range providerRegistry {
		modelsFromProvider := provider.GetModels()
		modelsForAPI := make([]ModelDetail, len(modelsFromProvider))

		for i, m := range modelsFromProvider {
			modelsForAPI[i] = ModelDetail{
				Name:            m.Name,
				SupportedParams: m.SupportedParams,
				MaxWidth:        m.MaxWidth,
				MaxHeight:       m.MaxHeight,
			}
		}

		providerInfo := ProviderInfo{
			Provider: name,
			Models:   modelsForAPI,
		}
		availableProviders = append(availableProviders, providerInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(availableProviders)
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB
		http.Error(w, "Could not parse multipart form", http.StatusBadRequest)
		return
	}

	// --- 1. Parse and Validate Input ---
	fullModelName := r.FormValue("model")
	parts := strings.SplitN(fullModelName, "/", 2)
	if len(parts) != 2 {
		http.Error(w, "Invalid model format. Expected 'provider/model_name'", http.StatusBadRequest)
		return
	}
	providerName, modelName := parts[0], parts[1]

	provider, ok := providerRegistry[providerName]
	if !ok {
		http.Error(w, fmt.Sprintf("Provider '%s' not found or not configured", providerName), http.StatusBadRequest)
		return
	}

	width, _ := strconv.Atoi(r.FormValue("width"))
	height, _ := strconv.Atoi(r.FormValue("height"))
	if width == 0 {
		width = 1024
	}
	if height == 0 {
		height = 1024
	}

	input := providers.GenerationInput{
		Prompt: r.FormValue("prompt"),
		Model:  modelName,
		Width:  width,
		Height: height,
		Seed:   rand.Int63n(1000000), // Default seed, max 6 digits
	}

	// Parse optional parameters
	if stepsStr := r.FormValue("steps"); stepsStr != "" {
		if steps, err := strconv.Atoi(stepsStr); err == nil {
			input.Steps = steps
		}
	}
	if seedStr := r.FormValue("seed"); seedStr != "" {
		if seed, err := strconv.ParseInt(seedStr, 10, 64); err == nil {
			input.Seed = seed
		}
	}

	// --- 2. Handle Image Input ---
	var tempImageID string // To store the ID of a temporarily uploaded image
	var providedImageBytes []byte
	var providedImageFilename string = "image.png" // Default filename

	// Try to get image from file upload first
	file, handler, err := r.FormFile("image")
	if err != nil && err != http.ErrMissingFile {
		http.Error(w, "Could not retrieve image from form", http.StatusBadRequest)
		return
	}

	if err == nil { // Image file was provided
		defer file.Close()
		providedImageBytes, _ = io.ReadAll(file)
		providedImageFilename = handler.Filename
	} else {
		// If no file, check for an image URL
		imageURL := r.FormValue("imageUrl")
		if imageURL != "" {
			log.Printf("Downloading image from provided URL: %s", imageURL)
			// Use the new shared DownloadFile function
			downloadedBytes, _, err := providers.DownloadFile(imageURL) // We don't need the content type here
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to download image from URL: %v", err), http.StatusBadRequest)
				return
			}
			providedImageBytes = downloadedBytes
		}
	}

	if len(providedImageBytes) > 0 {
		// --- 2a. Process Input Image (Resize and Compress) ---
		inputSizeLimit, _ := strconv.Atoi(r.FormValue("input_size_limit"))
		if inputSizeLimit == 0 {
			inputSizeLimit = 1024 // Default value
		}

		processedBytes, err := processImage(providedImageBytes, uint(inputSizeLimit))
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to process image: %v", err), http.StatusInternalServerError)
			return
		}
		providedImageBytes = processedBytes
		providedImageFilename = strings.TrimSuffix(providedImageFilename, ".png") + ".jpg" // Change extension to jpg

		input.ImageBytes = providedImageBytes

		// If the provider requires a URL, upload the image to the host first.
		if provider.RequiresImageURL() {
			if imageHostClient == nil {
				http.Error(w, "Image hosting is not configured, cannot process image for this provider", http.StatusInternalServerError)
				return
			}
			log.Println("Provider requires URL, uploading temporary image...")
			uploadResp, err := imageHostClient.UploadImage(providedImageBytes, providedImageFilename)
			if err != nil {
				errStr := fmt.Sprintf("Failed to upload temporary image: %v", err)
				log.Println(errStr)
				http.Error(w, errStr, http.StatusInternalServerError)
				return
			}
			input.ImageURL = uploadResp.Links.Direct
			tempImageID = uploadResp.ImageID
			log.Printf("Temporary image uploaded: %s (ID: %s)", input.ImageURL, tempImageID)

			// Defer the deletion of the temporary image.
			// This ensures it runs even if the provider call fails.
			defer func() {
				if tempImageID != "" {
					log.Printf("Deleting temporary image with ID: %s", tempImageID)
					if err := imageHostClient.DeleteImage(tempImageID); err != nil {
						log.Printf("Warning: failed to delete temporary image %s: %v", tempImageID, err)
					}
				}
			}()
		}
	}

	// --- 3. Call the Provider ---
	log.Printf("Calling provider '%s' with model '%s'", providerName, modelName)
	output, err := provider.Generate(input)
	if err != nil {
		errStr := fmt.Sprintf("Error from provider '%s': %v", providerName, err)
		log.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return // The deferred deletion will still run
	}

	// --- 4. Handle Temporary Image Deletion ---
	// The deletion is now handled by the deferred function call.
	// The explicit deletion block is no longer needed here.

	// --- 5. Process and Return Final Image ---
	finalImageBytes := output.ImageBytes
	// The logic for downloading from a provider's URL is now removed,
	// as all providers are expected to return image bytes directly.

	if len(finalImageBytes) == 0 {
		http.Error(w, "Provider did not return any image data", http.StatusInternalServerError)
		return
	}

	// Convert the final image to WebP for consistency and smaller size.
	webpBytes, err := convertToWebP(finalImageBytes)
	if err != nil {
		// If conversion fails, log the error but proceed with the original image.
		log.Printf("Warning: failed to convert image to WebP: %v. Using original format.", err)
		webpBytes = finalImageBytes // Fallback to original bytes
	} else {
		log.Printf("Successfully converted final image to WebP. Original size: %d, WebP size: %d", len(finalImageBytes), len(webpBytes))
	}

	// Generate a filename for potential local saving or content disposition header.
	now := time.Now()
	randomSuffix := rand.Intn(1000)
	finalFilename := fmt.Sprintf("%s_%03d.webp", now.Format("2006_0102_150405"), randomSuffix)
	localFilepath := fmt.Sprintf("images/%s", finalFilename)

	// Save the (potentially converted) image locally, if enabled.
	saveLocalCopy := os.Getenv("SAVE_LOCAL_COPY")
	if strings.ToLower(saveLocalCopy) != "false" {
		if err := os.WriteFile(localFilepath, webpBytes, 0644); err != nil {
			log.Printf("Warning: failed to save final image locally to %s: %v", localFilepath, err)
		} else {
			log.Printf("Successfully saved final image to %s", localFilepath)
		}
	} else {
		log.Println("Local save is disabled; skipping writing file to disk.")
	}

	// --- 6. Decide How to Return the Image ---
	uploadToHost := strings.ToLower(os.Getenv("UPLOAD_TO_IMAGE_HOST")) != "false"

	if !uploadToHost {
		// Return image data directly
		log.Println("UPLOAD_TO_IMAGE_HOST is false, returning image data directly.")
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", finalFilename))
		w.Write(webpBytes)
		log.Println("Successfully returned final image data to client.")
		return
	}

	// --- 7. Upload and Return URL (Default Behavior) ---
	if imageHostClient == nil {
		errStr := "Image hosting is not configured, cannot return final image URL. Set UPLOAD_TO_IMAGE_HOST=false to return image data directly."
		log.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return
	}

	log.Println("Uploading final image to image host...")
	finalUpload, err := imageHostClient.UploadImage(webpBytes, localFilepath)
	if err != nil {
		errStr := fmt.Sprintf("Failed to upload final image: %v", err)
		log.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"imageUrl": finalUpload.Links.Direct,
	})
	log.Printf("Successfully returned final image URL to client: %s", finalUpload.Links.Direct)
}

// processImage resizes and compresses an image.
func processImage(imageBytes []byte, sizeLimit uint) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Resize if either dimension exceeds the limit
	if uint(width) > sizeLimit || uint(height) > sizeLimit {
		log.Printf("Resizing image from %dx%d to fit within %dpx", width, height, sizeLimit)
		if width > height {
			img = resize.Resize(sizeLimit, 0, img, resize.Lanczos3)
		} else {
			img = resize.Resize(0, sizeLimit, img, resize.Lanczos3)
		}
	}

	// Compress to JPEG
	buf := new(bytes.Buffer)
	// Use a quality of 85 for a good balance between size and quality.
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("failed to encode image to JPEG: %w", err)
	}

	log.Printf("Image processed. Original size: %d bytes, New size: %d bytes", len(imageBytes), buf.Len())

	return buf.Bytes(), nil
}

// mimeTypeToExt maps a MIME type to a file extension.
func mimeTypeToExt(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		// Default to .png if the type is unknown or not an image.
		// We could also check for "image/" prefix.
		log.Printf("Unknown MIME type '%s', defaulting to .png", mimeType)
		return ".png"
	}
}

// convertToWebP takes image bytes, decodes them, and re-encodes as WebP.
func convertToWebP(imageBytes []byte) ([]byte, error) {
	// Decode the image. image.Decode automatically detects the format.
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image for WebP conversion: %w", err)
	}

	// Encode the image to WebP.
	buf := new(bytes.Buffer)
	// The second parameter to Encode is the quality, from 0 to 100. 80 is a good default.
	if err := webp.Encode(buf, img, &webp.Options{Quality: 80}); err != nil {
		return nil, fmt.Errorf("failed to encode image to WebP: %w", err)
	}

	return buf.Bytes(), nil
}

func handleOptimizePrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Could not read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var requestData struct {
		Prompt string `json:"prompt"`
	}

	if err := json.Unmarshal(body, &requestData); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	originalPrompt := requestData.Prompt
	// Get the dreamifly provider. We assume it's always registered.
	provider, ok := providerRegistry["dreamifly"].(*providers.DreamiflyProvider)
	if !ok {
		http.Error(w, "Dreamifly provider is not available", http.StatusInternalServerError)
		return
	}

	optimizedPrompt, err := provider.OptimizePrompt(originalPrompt)
	if err != nil {
		errStr := fmt.Sprintf("Error from prompt optimization provider: %v", err)
		log.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return
	}

	responseData := map[string]string{
		"optimized_prompt": optimizedPrompt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseData)
}
