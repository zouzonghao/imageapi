package providers

// ModelCapabilities defines the specific capabilities of an AI model.
type ModelCapabilities struct {
	Name            string   `json:"name"`
	SupportedParams []string `json:"supported_params"`
	MaxWidth        int      `json:"max_width"`
	MaxHeight       int      `json:"max_height"`
}

// GenerationInput defines the standardized input for all AI providers.
type GenerationInput struct {
	Prompt     string
	ImageBytes []byte // User-provided image file bytes
	ImageURL   string // User-provided image URL (for providers that need it)
	Width      int
	Height     int
	Model      string // The specific model name, e.g., "stable-diffusion"
	Seed       int64
	Steps      int `json:"steps,omitempty"`
}

// GenerationOutput defines the standardized output from all AI providers.
type GenerationOutput struct {
	ImageBytes []byte // The generated image bytes
	ImageURL   string // URL of the generated image (if applicable)
	Format     string // The format of the image, e.g., "png", "jpeg"
}

// ImageProvider is the interface that all AI providers must implement.
type ImageProvider interface {
	// Generate an image based on the provided input.
	Generate(input GenerationInput) (*GenerationOutput, error)
	// GetName returns the name of the provider (e.g., "dreamifly").
	GetName() string
	// GetModels returns a list of models supported by the provider and their capabilities.
	GetModels() []ModelCapabilities
	// RequiresImageURL returns true if the provider needs an image URL
	// instead of image bytes for image-to-image tasks.
	RequiresImageURL() bool
}
