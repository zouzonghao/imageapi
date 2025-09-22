//go:build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // Import for JPEG decoding
	_ "image/png"  // Import for PNG decoding
	"io/ioutil"
	"log"
	"os"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
)

func main() {
	inputFile := flag.String("i", "", "Input file path (webp, png, or jpg)")
	outputFile := flag.String("o", "output_interlaced_pixelated.webp", "Output file path")
	factor := flag.Float64("factor", 0.1, "Pixelation factor (e.g., 0.1 for 10% size)")
	flag.Parse()

	if *inputFile == "" {
		fmt.Println("Input file is required. Use -i flag.")
		os.Exit(1)
	}

	// Read the input file
	data, err := ioutil.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	// Decode the image. It will automatically detect the format (webp, png, or jpeg).
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		log.Fatalf("Failed to decode image: %v", err)
	}

	// Create a new RGBA image to draw on
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, image.Point{}, draw.Src)

	// Get image dimensions
	iw := bounds.Dx()
	ih := bounds.Dy()
	boxHeight := ih / 20

	// Process each stripe individually
	for i := 0; i < 20; i++ {
		var x, y, w, h int
		y = (i + 1) * boxHeight
		w = iw / 2
		h = boxHeight

		if i%2 == 0 {
			x = 0
		} else {
			x = iw / 2
		}

		// Adjust for the last box to fill the image
		if i == 19 {
			h = ih - y
		}

		// Define the rectangle for the current stripe
		stripeRect := image.Rect(x, y, x+w, y+h)

		// Crop the stripe from the original image
		croppedStripe := imaging.Crop(img, stripeRect)

		// --- Performance Optimization: Resize Down and Up ---
		// Calculate the new small dimensions
		smallW := int(float64(croppedStripe.Bounds().Dx()) * *factor)
		smallH := int(float64(croppedStripe.Bounds().Dy()) * *factor)
		if smallW < 1 {
			smallW = 1
		}
		if smallH < 1 {
			smallH = 1
		}

		// Resize down to a very small image
		smallImg := imaging.Resize(croppedStripe, smallW, smallH, imaging.NearestNeighbor)

		// Resize back up to the original stripe size
		pixelatedStripe := imaging.Resize(smallImg, croppedStripe.Bounds().Dx(), croppedStripe.Bounds().Dy(), imaging.NearestNeighbor)

		// Draw the pixelated stripe back onto the main image
		draw.Draw(rgba, stripeRect, pixelatedStripe, image.Point{}, draw.Src)
	}

	// Encode the image to webp
	outputData, err := webp.EncodeRGBA(rgba, 100.0)
	if err != nil {
		log.Fatalf("Failed to encode webp image: %v", err)
	}

	// Write the output file
	err = ioutil.WriteFile(*outputFile, outputData, 0644)
	if err != nil {
		log.Fatalf("Failed to write output file: %v", err)
	}

	fmt.Printf("Successfully created interlaced pixelated image: %s\n", *outputFile)
}
