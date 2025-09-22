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

	"imageapi/imagehost"
	"imageapi/middleware"
	"imageapi/providers"

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

	// Initialize the session store
	middleware.InitSessionStore()

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

	// Serve the index page, protected by authentication
	http.Handle("/", middleware.WebAuthMiddleware(http.HandlerFunc(serveIndex)))

	// Authentication routes
	http.HandleFunc("/login", serveLogin)
	http.HandleFunc("/auth/login", handleLogin)
	http.HandleFunc("/auth/logout", handleLogout)

	// Handle the API requests
	http.HandleFunc("/api/generate", handleGenerate)
	http.HandleFunc("/api/models", handleGetModels)
	http.HandleFunc("/api/optimize-prompt", handleOptimizePrompt)

	// External v1 API routes, protected by API Key
	apiV1 := http.NewServeMux()
	apiV1.HandleFunc("/api/v1/models", handleAPIGetModels)
	apiV1.HandleFunc("/api/v1/generate", handleAPIGenerate)
	http.Handle("/api/v1/", middleware.APIKeyAuthMiddleware(apiV1))

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

func serveLogin(w http.ResponseWriter, r *http.Request) {
	// If user is already logged in, redirect to home.
	session, _ := middleware.Store.Get(r, middleware.SessionName)
	if auth, ok := session.Values[middleware.UserSessionKey].(bool); ok && auth {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.ServeFile(w, r, "templates/login.html")
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	webPassword := os.Getenv("WEB_PASSWORD")
	if webPassword == "" {
		// This case should ideally not be reached if the middleware is correctly bypassed.
		http.Error(w, "Web authentication is not enabled on the server.", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	if password != webPassword {
		// Redirect back to login page with an error message.
		http.Redirect(w, r, "/login?error=invalid_password", http.StatusFound)
		return
	}

	session, _ := middleware.Store.Get(r, middleware.SessionName)
	session.Values[middleware.UserSessionKey] = true
	if err := session.Save(r, w); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := middleware.Store.Get(r, middleware.SessionName)
	// Set authenticated to false
	session.Values[middleware.UserSessionKey] = false
	session.Options.MaxAge = -1 // Expire the cookie immediately
	if err := session.Save(r, w); err != nil {
		log.Printf("Error saving session on logout: %v", err)
	}
	http.Redirect(w, r, "/login", http.StatusFound)
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
				Name:            fmt.Sprintf("%s/%s", name, m.Name),
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
	providerName, modelName, err := providers.ParseModelName(fullModelName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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
	// Specific model validation: Check if models that require an image have one.
	if (fullModelName == "Dreamifly/Flux-Kontext" || fullModelName == "Dreamifly/Qwen-Image-Edit") && len(input.ImageBytes) == 0 {
		errStr := fmt.Sprintf("Model '%s' requires an image", fullModelName)
		log.Println(errStr)
		http.Error(w, errStr, http.StatusBadRequest)
		return
	}

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
	provider, ok := providerRegistry["Dreamifly"].(*providers.DreamiflyProvider)
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

// --- V1 API Handlers ---

// handleAPIGetModels serves the list of available models for the external API.
// It reuses the same logic as the internal model handler but is exposed on a new, versioned endpoint.
func handleAPIGetModels(w http.ResponseWriter, r *http.Request) {
	handleGetModels(w, r)
}

// APIGenerateRequest defines the expected JSON structure for the v1 generate endpoint.
type APIGenerateRequest struct {
	Prompt   string `json:"prompt"`
	ImageURL string `json:"image_url"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Model    string `json:"model"`
	Seed     int64  `json:"seed,omitempty"`
	Steps    int    `json:"steps,omitempty"`
}

// APIGenerateResponse defines the JSON structure for the v1 generate endpoint response.
type APIGenerateResponse struct {
	Status   string `json:"status"`
	ImageURL string `json:"image_url,omitempty"`
	Error    string `json:"error,omitempty"`
}

// handleAPIGenerate handles image generation requests from the external API.
func handleAPIGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Decode JSON Request
	var apiReq APIGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&apiReq); err != nil {
		http.Error(w, "Invalid JSON request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 2. Validate Input
	if apiReq.Prompt == "" || apiReq.Model == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: "'prompt' and 'model' fields are required"})
		return
	}

	providerName, modelName, err := providers.ParseModelName(apiReq.Model)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: err.Error()})
		return
	}

	provider, ok := providerRegistry[providerName]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: fmt.Sprintf("Provider '%s' not found or not configured", providerName)})
		return
	}

	// 3. Prepare Generation Input
	width, height := apiReq.Width, apiReq.Height
	if width == 0 {
		width = 1024
	}
	if height == 0 {
		height = 1024
	}

	input := providers.GenerationInput{
		Prompt: apiReq.Prompt,
		Model:  modelName,
		Width:  width,
		Height: height,
		Seed:   apiReq.Seed,
		Steps:  apiReq.Steps,
	}
	if input.Seed == 0 {
		input.Seed = rand.Int63n(1000000)
	}

	// 4. Handle Image Input (from URL)
	var providedImageBytes []byte
	if apiReq.ImageURL != "" {
		log.Printf("API: Downloading image from provided URL: %s", apiReq.ImageURL)
		downloadedBytes, _, err := providers.DownloadFile(apiReq.ImageURL)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: fmt.Sprintf("Failed to download image from URL: %v", err)})
			return
		}
		providedImageBytes = downloadedBytes
	}

	if len(providedImageBytes) > 0 {
		// Process the image (resize/compress)
		processedBytes, err := processImage(providedImageBytes, 1024) // Default 1024px limit for API
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: fmt.Sprintf("Failed to process image: %v", err)})
			return
		}
		input.ImageBytes = processedBytes

		// If the provider requires a URL, we must upload it.
		if provider.RequiresImageURL() {
			if imageHostClient == nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: "Image hosting is not configured, cannot process image for this provider"})
				return
			}
			log.Println("API: Provider requires URL, uploading temporary image...")
			uploadResp, err := imageHostClient.UploadImage(processedBytes, "api_input.jpg")
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: fmt.Sprintf("Failed to upload temporary image: %v", err)})
				return
			}
			input.ImageURL = uploadResp.Links.Direct
			// We don't delete this temp image for API calls, for simplicity.
			// A more robust implementation might have a cleanup worker.
		}
	}

	// 5. Call the Provider
	// Specific model validation: Check if models that require an image have one.
	if (apiReq.Model == "Dreamifly/Flux-Kontext" || apiReq.Model == "Dreamifly/Qwen-Image-Edit") && apiReq.ImageURL == "" {
		errStr := fmt.Sprintf("Model '%s' requires an 'image_url'", apiReq.Model)
		log.Printf("API: Validation Error: %s", errStr)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: errStr})
		return
	}

	log.Printf("API: Calling provider '%s' with model '%s'", providerName, modelName)
	output, err := provider.Generate(input)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: fmt.Sprintf("Error from provider '%s': %v", providerName, err)})
		return
	}

	if len(output.ImageBytes) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: "Provider did not return any image data"})
		return
	}

	// 6. Process and Upload Final Image (API calls always save and upload)
	webpBytes, err := convertToWebP(output.ImageBytes)
	if err != nil {
		log.Printf("Warning: failed to convert image to WebP: %v. Using original format.", err)
		webpBytes = output.ImageBytes
	}

	now := time.Now()
	randomSuffix := rand.Intn(1000)
	finalFilename := fmt.Sprintf("%s_%03d.webp", now.Format("2006_0102_150405"), randomSuffix)
	localFilepath := fmt.Sprintf("images/%s", finalFilename)

	// Save locally
	if err := os.WriteFile(localFilepath, webpBytes, 0644); err != nil {
		log.Printf("API Warning: failed to save final image locally to %s: %v", localFilepath, err)
	} else {
		log.Printf("API: Successfully saved final image to %s", localFilepath)
	}

	// Upload to image host
	if imageHostClient == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: "Image hosting is not configured, cannot return final image URL."})
		return
	}

	finalUpload, err := imageHostClient.UploadImage(webpBytes, localFilepath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIGenerateResponse{Status: "error", Error: fmt.Sprintf("Failed to upload final image: %v", err)})
		return
	}

	// 7. Return Success Response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIGenerateResponse{
		Status:   "success",
		ImageURL: finalUpload.Links.Direct,
	})
	log.Printf("API: Successfully returned final image URL to client: %s", finalUpload.Links.Direct)
}
