package tray

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// GenerateIcon creates a pure color circle icon and returns it as an ICO byte array.
func GenerateIcon(hexColor string) ([]byte, error) {
	col := parseHexColor(hexColor)

	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)

	// Draw a simple circle
	center := size / 2
	radius := (size / 2) - 2

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := x - center
			dy := y - center
			if dx*dx+dy*dy <= radius*radius {
				img.Set(x, y, col)
			}
		}
	}

	// Encode to PNG first
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, err
	}
	pngData := pngBuf.Bytes()

	// Wrap PNG inside an ICO container for Windows compatibility
	var icoBuf bytes.Buffer
	
	// ICO Header (6 bytes)
	// Reserved (2), Type (2) 1=ICO, Count (2)
	icoBuf.Write([]byte{0, 0, 1, 0, 1, 0})
	
	// Directory Entry (16 bytes)
	icoBuf.WriteByte(byte(size)) // Width
	icoBuf.WriteByte(byte(size)) // Height
	icoBuf.WriteByte(0)          // Color palette count
	icoBuf.WriteByte(0)          // Reserved
	
	// Color planes (2 bytes)
	binary.Write(&icoBuf, binary.LittleEndian, uint16(1))
	// Bits per pixel (2 bytes)
	binary.Write(&icoBuf, binary.LittleEndian, uint16(32))
	// Image data size (4 bytes)
	binary.Write(&icoBuf, binary.LittleEndian, uint32(len(pngData)))
	// Image data offset (4 bytes)
	binary.Write(&icoBuf, binary.LittleEndian, uint32(6+16))
	
	// Image data
	icoBuf.Write(pngData)

	return icoBuf.Bytes(), nil
}

func parseHexColor(s string) color.RGBA {
	c := color.RGBA{A: 255}
	if s == "" || s[0] != '#' {
		return c
	}
	var r, g, b uint8
	switch len(s) {
	case 7:
		fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	case 4:
		fmt.Sscanf(s, "#%1x%1x%1x", &r, &g, &b)
		r *= 17
		g *= 17
		b *= 17
	}
	c.R = r
	c.G = g
	c.B = b
	return c
}
