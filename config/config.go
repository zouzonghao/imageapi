package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// APIKeys holds the API keys for various services.
type APIKeys struct {
	NodeImage      string `json:"NODEIMAGE_API_KEY"`
	FalAI          string `json:"FAL_API_KEY"`
	ModelScope     string `json:"MODELSCOPE_API_KEY"`
	PollinationsAI string `json:"POLLINATIONS_AI_API_KEY"`
	ImageAPI       string `json:"IMAGEAPI_API_KEY"`
}

// CloudflareCredentials holds the credentials for Cloudflare.
type CloudflareCredentials struct {
	AccountID string `json:"CLOUDFLARE_ACCOUNT_ID"`
	APIToken  string `json:"CLOUDFLARE_API_TOKEN"`
}

// Settings holds optional application settings.
type Settings struct {
	SaveLocalCopy     bool   `json:"SAVE_LOCAL_COPY"`
	UploadToImageHost bool   `json:"UPLOAD_TO_IMAGE_HOST"`
	WebPassword       string `json:"WEB_PASSWORD"`
	SessionSecret     string `json:"SESSION_SECRET"`
}

// Config holds the entire application configuration.
type Config struct {
	APIKeys               APIKeys               `json:"API_KEYS"`
	CloudflareCredentials CloudflareCredentials `json:"CLOUDFLARE_CREDENTIALS"`
	Settings              Settings              `json:"SETTINGS"`
}

// AppConfig is the global configuration instance.
var AppConfig *Config

// LoadConfig loads the configuration from defaults, conf.json, .env, and environment variables.
func LoadConfig() {
	// 1. Set default values
	AppConfig = &Config{
		Settings: Settings{
			SaveLocalCopy:     true,
			UploadToImageHost: true,
			SessionSecret:     "a_very_long_and_random_secret_string",
		},
	}

	// 2. Load from conf.json
	file, err := os.Open("conf.json")
	if err == nil {
		defer file.Close()
		decoder := json.NewDecoder(file)
		if err := decoder.Decode(AppConfig); err != nil {
			log.Printf("Warning: Could not decode conf.json: %v", err)
		} else {
			log.Println("Loaded configuration from conf.json")
		}
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: Could not open conf.json: %v", err)
	}

	// 3. Load from .env file (will override conf.json)
	godotenv.Load()

	// 4. Load from environment variables (will override everything)
	loadFromEnv()

	log.Println("Configuration loaded successfully.")
}

// loadFromEnv loads configuration from environment variables, overriding existing values.
func loadFromEnv() {
	// API Keys
	if key := os.Getenv("NODEIMAGE_API_KEY"); key != "" {
		AppConfig.APIKeys.NodeImage = key
	}
	if key := os.Getenv("FAL_API_KEY"); key != "" {
		AppConfig.APIKeys.FalAI = key
	}
	if key := os.Getenv("MODELSCOPE_API_KEY"); key != "" {
		AppConfig.APIKeys.ModelScope = key
	}
	if key := os.Getenv("POLLINATIONS_AI_API_KEY"); key != "" {
		AppConfig.APIKeys.PollinationsAI = key
	}
	if key := os.Getenv("IMAGEAPI_API_KEY"); key != "" {
		AppConfig.APIKeys.ImageAPI = key
	}

	// Cloudflare
	if id := os.Getenv("CLOUDFLARE_ACCOUNT_ID"); id != "" {
		AppConfig.CloudflareCredentials.AccountID = id
	}
	if token := os.Getenv("CLOUDFLARE_API_TOKEN"); token != "" {
		AppConfig.CloudflareCredentials.APIToken = token
	}

	// Settings
	if val := os.Getenv("SAVE_LOCAL_COPY"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			AppConfig.Settings.SaveLocalCopy = b
		}
	}
	if val := os.Getenv("UPLOAD_TO_IMAGE_HOST"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			AppConfig.Settings.UploadToImageHost = b
		}
	}
	if pass := os.Getenv("WEB_PASSWORD"); pass != "" {
		AppConfig.Settings.WebPassword = pass
	}
	if secret := os.Getenv("SESSION_SECRET"); secret != "" {
		AppConfig.Settings.SessionSecret = secret
	}
}
