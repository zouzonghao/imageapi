//go:build ignore

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg" // Import for decoding JPEGs
	_ "image/png"  // Import for decoding PNGs
	"io/ioutil"    // For reading image file (Go 1.15 and older)
	"log"
	"net/http"
	"os"
	"strconv"

	_ "github.com/chai2010/webp" // Import for decoding WebP

	"github.com/joho/godotenv"
)

// API_ENDPOINT 定义 Cloudflare AI API 的基础 URL
const API_ENDPOINT = "https://api.cloudflare.com/client/v4/accounts/%s/ai/run/@cf/runwayml/stable-diffusion-v1-5-inpainting"

// Img2ImgRequest 结构体定义了发送到 Cloudflare AI API 的请求体
type Img2ImgRequest struct {
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt,omitempty"` // omitempty: if empty, don't include in JSON
	Height         int     `json:"height,omitempty"`
	Width          int     `json:"width,omitempty"`
	Image          []int   `json:"image"` // Input image as an array of bytes
	Mask           []int   `json:"mask"`  // Inpainting mask as an array of bytes
	NumSteps       int     `json:"num_steps,omitempty"`
	Strength       float64 `json:"strength,omitempty"`
	Guidance       float64 `json:"guidance,omitempty"`
	Seed           int     `json:"seed,omitempty"`
}

// Img2ImgResponse 结构体定义了从 Cloudflare AI API 返回的响应体
type Img2ImgResponse struct {
	Result struct {
		ImageB64 string `json:"image_b64"`
	} `json:"result"`
	Success  bool     `json:"success"`
	Errors   []string `json:"errors"`
	Messages []string `json:"messages"`
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, relying on system environment variables.")
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <input_image.png>")
		os.Exit(1)
	}
	inputImagePath := os.Args[1]

	// 1. 读取 API 密钥和账户 ID
	cloudflareAccountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
	cloudflareAPIToken := os.Getenv("CLOUDFLARE_API_TOKEN")

	if cloudflareAccountID == "" || cloudflareAPIToken == "" {
		log.Fatal("Error: CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN environment variables must be set.")
	}

	// 2. 加载输入图片
	// 2.1 直接读取原始图片文件的字节
	imageBytes, err := ioutil.ReadFile(inputImagePath)
	if err != nil {
		log.Fatalf("Error reading image file: %v", err)
	}

	// 2.2 解码图片以获取尺寸信息
	img, _, err := image.Decode(bytes.NewReader(imageBytes))
	if err != nil {
		log.Fatalf("Error decoding image for dimensions: %v", err)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	fmt.Printf("Input image dimensions: %dx%d\n", width, height)

	// 3. 用户输入重绘区域坐标
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter coordinates for inpainting (top-left x1, y1 and bottom-right x2, y2).")
	fmt.Printf("Coordinates must be within 0 and %d for width, and 0 and %d for height.\n", width-1, height-1)

	var x1, y1, x2, y2 int

	x1, y1 = readCoords(reader, "top-left", width, height)
	x2, y2 = readCoords(reader, "bottom-right", width, height)

	// 确保坐标顺序正确 (x1 <= x2, y1 <= y2)
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	fmt.Printf("Inpainting area: (%d,%d) to (%d,%d)\n", x1, y1, x2, y2)

	// 4. 生成 Mask 字节数组
	maskBytes := generateMask(width, height, x1, y1, x2, y2)

	// 4.1 将字节切片 ([]uint8) 转换为整数切片 ([]int) 以匹配 API 期望的 JSON 数组格式
	imageInts := make([]int, len(imageBytes))
	for i, b := range imageBytes {
		imageInts[i] = int(b)
	}
	maskInts := make([]int, len(maskBytes))
	for i, b := range maskBytes {
		maskInts[i] = int(b)
	}

	// 5. 构造 API 请求体
	requestPayload := Img2ImgRequest{
		Prompt:         "a detailed painting of a futuristic robot where the original image had a tree.", // 用户可以自行修改或添加输入
		NegativePrompt: "blurry, low quality, bad aesthetics, deformed",
		Height:         height, // 使用原始图片的高度
		Width:          width,  // 使用原始图片的宽度
		Image:          imageInts,
		Mask:           maskInts,
		NumSteps:       20,
		Strength:       0.7, // 0.0 makes output closer to input, 1.0 makes it closer to prompt
		Guidance:       7.5,
		// Seed:      // Optional: for reproducibility
	}

	jsonPayload, err := json.Marshal(requestPayload)
	if err != nil {
		log.Fatalf("Error marshalling request payload: %v", err)
	}

	// 6. 发送 HTTP 请求
	apiURL := fmt.Sprintf(API_ENDPOINT, cloudflareAccountID)
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Fatalf("Error creating HTTP request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+cloudflareAPIToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("API request failed with status code %d: %s", resp.StatusCode, string(body))
	}

	// 7. 处理 API 响应
	var apiResponse Img2ImgResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		log.Fatalf("Error unmarshalling API response: %v", err)
	}

	if !apiResponse.Success {
		log.Fatalf("API returned an error: %v", apiResponse.Errors)
	}

	if apiResponse.Result.ImageB64 == "" {
		log.Fatal("API response did not contain a generated image.")
	}

	// 解码返回的 Base64 图片
	decodedImageBytes, err := base64.StdEncoding.DecodeString(apiResponse.Result.ImageB64)
	if err != nil {
		log.Fatalf("Error decoding returned image Base64 string: %v", err)
	}

	// 8. 保存返回的图片
	outputFileName := "inpainted_image.png"
	err = ioutil.WriteFile(outputFileName, decodedImageBytes, 0644)
	if err != nil {
		log.Fatalf("Error saving inpainted image: %v", err)
	}

	fmt.Printf("Successfully inpainted image and saved to %s\n", outputFileName)
}

// readCoords 辅助函数，用于读取用户输入的坐标
func readCoords(reader *bufio.Reader, prompt string, maxWidth, maxHeight int) (int, int) {
	for {
		fmt.Printf("Enter %s X coordinate: ", prompt)
		xStr, _ := reader.ReadString('\n')
		x, err := strconv.Atoi(trimNewline(xStr))
		if err != nil || x < 0 || x >= maxWidth {
			fmt.Printf("Invalid X coordinate. Must be an integer between 0 and %d.\n", maxWidth-1)
			continue
		}

		fmt.Printf("Enter %s Y coordinate: ", prompt)
		yStr, _ := reader.ReadString('\n')
		y, err := strconv.Atoi(trimNewline(yStr))
		if err != nil || y < 0 || y >= maxHeight {
			fmt.Printf("Invalid Y coordinate. Must be an integer between 0 and %d.\n", maxHeight-1)
			continue
		}
		return x, y
	}
}

// trimNewline 辅助函数，用于去除从 bufio.ReadString 读取的换行符
func trimNewline(s string) string {
	return s[:len(s)-1] // Remove '\n' or '\r\n'
}

// generateMask 根据用户提供的坐标生成一个单通道的 mask 字节数组。
// mask 区域为 255 (白色), 外部区域为 0 (黑色)。
func generateMask(width, height, x1, y1, x2, y2 int) []uint8 {
	mask := make([]uint8, width*height) // 单通道灰度图每个像素一个字节

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := y*width + x
			if x >= x1 && x <= x2 && y >= y1 && y <= y2 {
				mask[index] = 255 // 白色区域，表示需要重绘
			} else {
				mask[index] = 0 // 黑色区域，表示保持不变
			}
		}
	}
	return mask
}
