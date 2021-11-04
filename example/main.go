package main

import (
	"fmt"
	"image/color"
	"log"
	"os"

	qrcode "github.com/RashadAnsari/go-qrcode"
)

func main() {
	qr, err := qrcode.New("https://rashadansari.github.io", qrcode.High)
	if err != nil {
		log.Fatal(err.Error())
	}

	opacity := 100
	a := (float64(opacity) / float64(100)) * float64(255)
	qr.ForegroundColor = color.RGBA{R: 255, G: 0, B: 0, A: uint8(a)}

	writeToFile("qr.png", qr.PNG)
	writeToFile("qr.jpeg", qr.JPEG)
	writeToFile("qr.svg", qr.SVG)
	writeToFile("qr.pdf", qr.PDF)

	qr.Base64 = true

	stdoutBase64(qr.PNG)
	fmt.Println("----------")
	stdoutBase64(qr.JPEG)
	fmt.Println("----------")
	stdoutBase64(qr.PDF)
	fmt.Println("----------")
	stdoutBase64(qr.SVG)
}

func writeToFile(fileName string, FormatFunc func(_ int) ([]byte, error)) {
	size := 500
	fileMode := os.FileMode(0644)

	bytes, err := FormatFunc(size)
	if err != nil {
		log.Fatal(err.Error())
	}

	if err := os.WriteFile(fileName, bytes, fileMode); err != nil {
		log.Fatal(err.Error())
	}
}

func stdoutBase64(FormatFunc func(_ int) ([]byte, error)) {
	size := 500

	bytes, err := FormatFunc(size)
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println(string(bytes))
}
