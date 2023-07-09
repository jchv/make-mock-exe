// make-mock-exe by John Chadwick <john@jchw.io>
//
// To the extent possible under law, the person who associated CC0 with
// make-mock-exe has waived all copyright and related or neighboring rights
// to make-mock-exe.
//
// You should have received a copy of the CC0 legalcode along with this
// work.  If not, see <http://creativecommons.org/publicdomain/zero/1.0/>.

package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
)

type BitmapInfoHeaderV3 struct {
	Size            uint32
	Width           int32
	Height          int32
	Planes          int16
	BPP             int16
	Compression     uint32
	ImageSize       uint32
	XPixelsPerMeter int32
	YPixelsPerMeter int32
	ColorsUsed      uint32
	ColorsImportant uint32
}

const SizeOfBitmapInfoHeaderV3 = 40

type DIB struct {
	width, height  int
	bpp, numColors int
	palette        color.Palette

	headerSize         int
	paletteSize        int
	scanlineStride     int
	maskScanlineStride int

	size int

	image, mask image.Image
}

func NewDIB(img image.Image, mask image.Image, nbit int) (*DIB, error) {
	w := DIB{image: img, mask: mask}
	w.width = img.Bounds().Dx()
	w.height = img.Bounds().Dy()
	w.bpp = 24
	if palette, ok := img.ColorModel().(color.Palette); ok {
		w.palette = palette
		w.numColors = len(palette)
		switch {
		case w.numColors <= 2:
			w.bpp = 1
		case w.numColors <= 16:
			w.bpp = 4
		default:
			w.bpp = 8
		}
	} else {
	checkAlpha:
		for y := 0; y < w.height; y++ {
			for x := 0; x < w.width; x++ {
				_, _, _, a := img.At(x, y).RGBA()
				if a < 0xffff {
					w.bpp = 32
					break checkAlpha
				}
			}
		}
		// Downgrade to 16bpp if possible.
		if w.bpp == 24 && nbit == 16 {
			w.bpp = 16
		}
	}
	if w.bpp != nbit {
		return nil, fmt.Errorf("expected %d bits, got %d", nbit, w.bpp)
	}

	w.headerSize = SizeOfBitmapInfoHeaderV3
	w.paletteSize = 4 * w.numColors
	w.scanlineStride = bppstride(w.width, w.bpp)
	w.maskScanlineStride = bppstride(w.width, 1)
	w.size = w.headerSize + w.paletteSize + w.scanlineStride*w.height + w.maskScanlineStride*w.height
	return &w, nil
}

func (d *DIB) IconGroupWidth() int {
	if d.width >= 256 {
		return 0
	}
	return d.width
}

func (d *DIB) IconGroupHeight() int {
	if d.height >= 256 {
		return 0
	}
	return d.height
}

func (d *DIB) Write(w io.Writer) {
	iconScanline := make([]byte, d.scanlineStride)
	iconMaskScanline := make([]byte, d.maskScanlineStride)

	// Icon data
	must(binary.Write(w, binary.LittleEndian, BitmapInfoHeaderV3{
		Size:            uint32(SizeOfBitmapInfoHeaderV3),
		Width:           int32(d.width),
		Height:          int32(d.height * 2),
		Planes:          1,
		BPP:             int16(d.bpp),
		Compression:     0,
		ImageSize:       0, // TODO
		XPixelsPerMeter: 2835,
		YPixelsPerMeter: 2835,
		ColorsUsed:      uint32(d.numColors),
		ColorsImportant: uint32(d.numColors),
	}), "writing icon dib header")

	dibPalette := make([][4]byte, len(d.palette))
	for i := range dibPalette {
		r, g, b, _ := d.palette[i].RGBA()
		dibPalette[i][0] = byte(b / 0x100)
		dibPalette[i][1] = byte(g / 0x100)
		dibPalette[i][2] = byte(r / 0x100)
		dibPalette[i][3] = 0
	}
	must(binary.Write(w, binary.LittleEndian, dibPalette), "writing dib palette")

	switch d.bpp {
	case 1:
		for y := d.height - 1; y >= 0; y-- {
			indexed := d.image.(*image.Paletted)
			i, x := 0, 0
			for ; i < d.width/8; i++ {
				iconScanline[i] = (indexed.ColorIndexAt(x+0, y)<<7 |
					indexed.ColorIndexAt(x+1, y)<<6 |
					indexed.ColorIndexAt(x+2, y)<<5 |
					indexed.ColorIndexAt(x+3, y)<<4 |
					indexed.ColorIndexAt(x+4, y)<<3 |
					indexed.ColorIndexAt(x+5, y)<<2 |
					indexed.ColorIndexAt(x+6, y)<<1 |
					indexed.ColorIndexAt(x+7, y))
				x += 8
			}
			if x < d.width {
				iconScanline[i] = 0
				for b := 7; x < d.width; x++ {
					iconScanline[i] |= indexed.ColorIndexAt(x, y) << b
					b--
				}
			}

			_, err := w.Write(iconScanline)
			must(err, "writing 1bpp scanline")
		}
	case 4:
		for y := d.height - 1; y >= 0; y-- {
			indexed := d.image.(*image.Paletted)
			i, x := 0, 0
			for ; i < d.width/2; i++ {
				iconScanline[i] = indexed.ColorIndexAt(x+0, y)<<4 | indexed.ColorIndexAt(x+1, y)
				x += 2
			}
			if d.width&1 != 0 {
				iconScanline[i] = indexed.ColorIndexAt(x, y) << 4
			}

			_, err := w.Write(iconScanline)
			must(err, "writing 4bpp scanline")
		}
	case 8:
		for y := d.height - 1; y >= 0; y-- {
			indexed := d.image.(*image.Paletted)
			for x := 0; x < d.width; x++ {
				iconScanline[x] = indexed.ColorIndexAt(x, y)
			}

			_, err := w.Write(iconScanline)
			must(err, "writing 8bpp scanline")
		}
	case 16:
		for y := d.height - 1; y >= 0; y-- {
			for x := 0; x < d.width; x++ {
				r, g, b, _ := d.image.At(x, y).RGBA()
				rgb555 := (r>>11)<<10 | (g>>11)<<5 | (b >> 11)
				iconScanline[x*2+0] = byte(rgb555 & 0xff)
				iconScanline[x*2+1] = byte(rgb555 >> 8)
			}

			_, err := w.Write(iconScanline)
			must(err, "writing 16bpp scanline")
		}
	case 24:
		for y := d.height - 1; y >= 0; y-- {
			for x := 0; x < d.width; x++ {
				r, g, b, _ := d.image.At(x, y).RGBA()
				iconScanline[x*3+0] = byte(b >> 8)
				iconScanline[x*3+1] = byte(g >> 8)
				iconScanline[x*3+2] = byte(r >> 8)
			}

			_, err := w.Write(iconScanline)
			must(err, "writing 24bpp scanline")
		}
	case 32:
		for y := d.height - 1; y >= 0; y-- {
			for x := 0; x < d.width; x++ {
				r, g, b, a := d.image.At(x, y).RGBA()
				iconScanline[x*4+0] = byte(b >> 8)
				iconScanline[x*4+1] = byte(g >> 8)
				iconScanline[x*4+2] = byte(r >> 8)
				iconScanline[x*4+3] = byte(a >> 8)
			}

			_, err := w.Write(iconScanline)
			must(err, "writing 32bpp scanline")
		}
	}

	for y := d.height - 1; y >= 0; y-- {
		i, x := 0, 0
		for ; i < d.width/8; i++ {
			iconMaskScanline[i] = (threshold(d.mask.At(x+0, y))<<7 |
				threshold(d.mask.At(x+1, y))<<6 |
				threshold(d.mask.At(x+2, y))<<5 |
				threshold(d.mask.At(x+3, y))<<4 |
				threshold(d.mask.At(x+4, y))<<3 |
				threshold(d.mask.At(x+5, y))<<2 |
				threshold(d.mask.At(x+6, y))<<1 |
				threshold(d.mask.At(x+7, y)))
			x += 8
		}
		if x < d.width {
			iconMaskScanline[i] = 0
			for b := 7; x < d.width; x++ {
				iconMaskScanline[i] |= threshold(d.mask.At(x, y)) << b
				b--
			}
		}

		_, err := w.Write(iconMaskScanline)
		must(err, "writing 1bpp mask scanline")
	}
}

func bppstride(w, bpp int) int {
	return (((w * bpp) + 31) &^ 31) / 8
}

func threshold(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	if r+g+b >= 0x18000 {
		return 1
	}
	return 0
}
