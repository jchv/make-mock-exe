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
	"embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"log"
	"os"
)

//go:embed asset/*
var fsys embed.FS

var (
	img1bpp  = loadpng("asset/1bpp.png")
	img4bpp  = loadpng("asset/4bpp.png")
	img8bpp  = loadpng("asset/8bpp.png")
	img16bpp = loadpng("asset/16bpp.png")
	img24bpp = loadpng("asset/24bpp.png")
	img32bpp = loadpng("asset/32bpp.png")
	imgMask  = loadpng("asset/mask.png")
)

// IconDirectoryEntry are entries of the GroupIconDirectory when stored on-disk.
type IconDirectoryEntry struct {
	Width       uint8
	Height      uint8
	ColorCount  uint8
	Reserved    uint8
	NumPlanes   uint16
	BitCount    uint16
	ImageSize   uint32
	ImageOffset uint32
}

const SizeOfIconDirectoryEntry = 16

const PEResourceDirOverhead = SizeOfResourceDirectoryTable +
	SizeOfResourceDirectoryEntry*2 +
	SizeOfResourceDirectoryTable +
	SizeOfResourceDirectoryEntry +
	SizeOfResourceDirectoryTable +
	SizeOfResourceDirectoryEntry +
	SizeOfResourceDataEntry +
	SizeOfResourceDataEntry +
	SizeOfResourceDirectoryTable +
	SizeOfResourceDirectoryEntry +
	SizeOfResourceDirectoryTable +
	SizeOfResourceDirectoryEntry +
	SizeOfResourceDataEntry +
	SizeOfGroupIconDirectory +
	SizeOfGroupIconDirectoryEntry

type EXEFormat int

const (
	NE16 EXEFormat = iota
	PE32
	PE32Plus
)

func main() {
	png2exe(create("out/ne16-1bpp.exe"), create("out/1bpp.ico"), img1bpp, imgMask, NE16, 1)
	png2exe(create("out/ne16-4bpp.exe"), create("out/4bpp.ico"), img4bpp, imgMask, NE16, 4)
	png2exe(create("out/ne16-8bpp.exe"), create("out/8bpp.ico"), img8bpp, imgMask, NE16, 8)
	png2exe(create("out/pe32-16bpp.exe"), create("out/16bpp.ico"), img16bpp, imgMask, PE32, 16)
	png2exe(create("out/pe32-24bpp.exe"), create("out/24bpp.ico"), img24bpp, imgMask, PE32, 24)
	png2exe(create("out/pe32-32bpp.exe"), create("out/32bpp.ico"), img32bpp, imgMask, PE32, 32)
	png2exe(create("out/pe32plus-32bpp.exe"), io.Discard, img32bpp, imgMask, PE32Plus, 32)
}

func loadpng(name string) image.Image {
	img, err := png.Decode(open(name))
	must(err, "decoding %q", name)
	return img
}

func open(name string) fs.File {
	f, err := fsys.Open(name)
	must(err, "opening %q", name)
	return f
}

func create(name string) *os.File {
	f, err := os.Create(name)
	must(err, "opening %q for write", name)
	return f
}

func png2exe(exeWriter io.Writer, icoWriter io.Writer, img image.Image, mask image.Image, exeFormat EXEFormat, nbit int) {
	dosHeader := ImageDOSHeader{
		Signature:     MZSignature,
		NewHeaderAddr: uint32(SizeOfImageDOSHeader),
	}
	dib, err := NewDIB(img, mask, nbit)
	must(err, "processing image")
	must(binary.Write(exeWriter, binary.LittleEndian, dosHeader), "writing DOS header")
	switch exeFormat {
	case NE16:
		ne16(exeWriter, icoWriter, dib)
	case PE32:
		pe32(exeWriter, dib)
		peresource(exeWriter, icoWriter, dib)
	case PE32Plus:
		pe32plus(exeWriter, dib)
		peresource(exeWriter, icoWriter, dib)
	}
}

func ne16(exeWriter io.Writer, icoWriter io.Writer, dib *DIB) {
	resourceTableSize := SizeOfNEResourceTableHeader + 3*SizeOfNEResourceTableEntry + 2*SizeOfNEResource
	residentNameTableSize := 4
	headerSize := SizeOfNEFileHeader + resourceTableSize + residentNameTableSize
	groupIconSize := SizeOfGroupIconDirectory + SizeOfGroupIconDirectoryEntry

	// These offsets are relative to the NE header
	resourceTableOffset := SizeOfNEFileHeader
	residentNameTableOffset := SizeOfNEFileHeader + resourceTableSize

	// These two offsets are relative to the beginning of the file
	groupIconOffset := SizeOfImageDOSHeader + headerSize
	iconOffset := SizeOfImageDOSHeader + headerSize + groupIconSize

	must(binary.Write(exeWriter, binary.LittleEndian, NEFileHeader{
		Signature:                 NESignature,
		OffsetOfResourceTable:     uint16(resourceTableOffset),
		OffsetOfResidentNameTable: uint16(residentNameTableOffset),
		ExecutableType:            2,
	}), "writing NE16 header")
	shift := 1
	must(binary.Write(exeWriter, binary.LittleEndian, NEResourceTableHeader{
		AlignmentShiftCount: uint16(shift),
	}), "writing NE16 resource table header")
	must(binary.Write(exeWriter, binary.LittleEndian, NEResourceTableEntry{
		TypeID:       ResourceIcon ^ 0x8000,
		NumResources: 1,
	}), "writing NE16 resource table icon entry")
	must(binary.Write(exeWriter, binary.LittleEndian, NEResource{
		DataOffsetShifted: uint16(iconOffset) >> shift,
		DataLength:        uint16(dib.size),
		Flags:             0x1c10,
		ResourceID:        0x8001,
	}), "writing NE16 resource icon resource")
	must(binary.Write(exeWriter, binary.LittleEndian, NEResourceTableEntry{
		TypeID:       ResourceGroupIcon ^ 0x8000,
		NumResources: 1,
	}), "writing NE16 resource group icon entry")
	must(binary.Write(exeWriter, binary.LittleEndian, NEResource{
		DataOffsetShifted: uint16(groupIconOffset) >> shift,
		DataLength:        uint16(groupIconSize),
		Flags:             0x1c10,
		ResourceID:        0x8001,
	}), "writing NE16 resource group icon resource")
	must(binary.Write(exeWriter, binary.LittleEndian, NEResourceTableEntry{}), "writing NE16 terminal resource entry")

	must(binary.Write(exeWriter, binary.LittleEndian, make([]byte, residentNameTableSize)), "writing blank resident name table")

	w := io.MultiWriter(exeWriter, icoWriter)

	// Group icon directory
	must(binary.Write(w, binary.LittleEndian, GroupIconDirectory{
		Type:  1,
		Count: 1,
	}), "writing group icon directory")

	// Group icon directory entry
	must(binary.Write(exeWriter, binary.LittleEndian, GroupIconDirectoryEntry{
		Width:      uint8(dib.IconGroupWidth()),
		Height:     uint8(dib.IconGroupHeight()),
		ColorCount: uint8(dib.numColors),
		NumPlanes:  1,
		BPP:        uint16(dib.bpp),
		ImageSize:  uint32(dib.size),
		ResourceID: 1,
	}), "writing group icon directory entry (exe)")

	must(binary.Write(icoWriter, binary.LittleEndian, IconDirectoryEntry{
		Width:       uint8(dib.IconGroupWidth()),
		Height:      uint8(dib.IconGroupHeight()),
		ColorCount:  uint8(dib.numColors),
		NumPlanes:   1,
		ImageSize:   uint32(dib.size),
		ImageOffset: uint32(SizeOfGroupIconDirectory + SizeOfIconDirectoryEntry),
	}), "writing icon directory entry (ico)")

	dib.Write(w)
}

func pe32(w io.Writer, dib *DIB) {
	resDirSize := PEResourceDirOverhead + dib.size
	optHeader := ImageOptionalHeaderPE32{
		Magic:               ImageNTOptionalHeaderPE32Magic,
		ImageBase:           0x400000,
		SectionAlignment:    0x1000,
		FileAlignment:       0x200,
		SizeOfImage:         uint32(0x200 + resDirSize),
		SizeOfHeaders:       0x200,
		Subsystem:           2,
		NumberOfRvaAndSizes: 15,
	}
	optHeader.DataDirectory[ImageDirectoryEntryResource] = ImageDataDirectory{
		VirtualAddress: 0x1000,
		Size:           uint32(resDirSize),
	}
	newHeader := ImageNTHeadersPE32{
		Signature: PESignature,
		FileHeader: ImageFileHeader{
			Machine:              ImageFileMachinei386,
			NumberOfSections:     1,
			SizeOfOptionalHeader: SizeOfImageOptionalHeaderPE32,
		},
		OptionalHeader: optHeader,
	}
	must(binary.Write(w, binary.LittleEndian, newHeader), "writing PE32 header")

	section := ImageSectionHeader{
		PhysicalAddressOrVirtualSize: 0x8000,
		VirtualAddress:               0x1000,
		SizeOfRawData:                uint32(resDirSize),
		PointerToRawData:             0x200,
		Characteristics:              ImageSectionCharacteristicsMemoryRead | ImageSectionCharacteristicsMemoryWrite | ImageSectionCharacteristicsContainsInitializedData,
	}
	copy(section.Name[:], ".rsrc")
	must(binary.Write(w, binary.LittleEndian, section), "writing section")

	currentOffset := SizeOfImageDOSHeader + SizeOfImageNTHeadersPE32 + SizeOfImageSectionHeader
	_, err := w.Write(make([]byte, 0x200-currentOffset))
	must(err, "writing padding to first section")
}

func pe32plus(w io.Writer, dib *DIB) {
	resDirSize := PEResourceDirOverhead + dib.size
	optHeader := ImageOptionalHeaderPE32Plus{
		Magic:               ImageNTOptionalHeaderPE32PlusMagic,
		ImageBase:           0x400000,
		SectionAlignment:    0x1000,
		FileAlignment:       0x200,
		SizeOfImage:         uint32(0x200 + resDirSize),
		SizeOfHeaders:       0x200,
		Subsystem:           2,
		NumberOfRvaAndSizes: 15,
	}
	optHeader.DataDirectory[ImageDirectoryEntryResource] = ImageDataDirectory{
		VirtualAddress: 0x1000,
		Size:           uint32(resDirSize),
	}
	newHeader := ImageNTHeadersPE32Plus{
		Signature: PESignature,
		FileHeader: ImageFileHeader{
			Machine:              ImageFileMachineAMD64,
			NumberOfSections:     1,
			SizeOfOptionalHeader: SizeOfImageOptionalHeaderPE32Plus,
		},
		OptionalHeader: optHeader,
	}
	must(binary.Write(w, binary.LittleEndian, newHeader), "writing PE32+ header")

	section := ImageSectionHeader{
		PhysicalAddressOrVirtualSize: 0x8000,
		VirtualAddress:               0x1000,
		SizeOfRawData:                uint32(resDirSize),
		PointerToRawData:             0x200,
		Characteristics:              ImageSectionCharacteristicsMemoryRead | ImageSectionCharacteristicsMemoryWrite | ImageSectionCharacteristicsContainsInitializedData,
	}
	copy(section.Name[:], ".rsrc")
	must(binary.Write(w, binary.LittleEndian, section), "writing section")

	currentOffset := SizeOfImageDOSHeader + SizeOfImageNTHeadersPE32Plus + SizeOfImageSectionHeader
	_, err := w.Write(make([]byte, 0x200-currentOffset))
	must(err, "writing padding to header")
}

func peresource(exeWriter io.Writer, icoWriter io.Writer, dib *DIB) {
	iconResDirOffset := SizeOfResourceDirectoryTable + SizeOfResourceDirectoryEntry*2
	iconResDir2Offset := iconResDirOffset + SizeOfResourceDirectoryTable + SizeOfResourceDirectoryEntry
	iconResDataEntryOffset := iconResDir2Offset + SizeOfResourceDirectoryTable + SizeOfResourceDirectoryEntry
	groupIconResDirOffset := iconResDataEntryOffset + SizeOfResourceDataEntry
	groupIconResDir2Offset := groupIconResDirOffset + SizeOfResourceDirectoryTable + SizeOfResourceDirectoryEntry
	groupIconDataEntryOffset := groupIconResDir2Offset + SizeOfResourceDirectoryTable + SizeOfResourceDirectoryEntry
	groupIconOffset := groupIconDataEntryOffset + SizeOfResourceDataEntry
	groupIconSize := SizeOfGroupIconDirectory + SizeOfGroupIconDirectoryEntry
	iconOffset := groupIconOffset + groupIconSize

	// Root directory
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryTable{
		NumIDEntries: 2,
		MajorVersion: 4,
	}), "writing root resource dir")
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryEntry{
		ID:     ResourceIcon,
		Offset: 0x80000000 | uint32(iconResDirOffset),
	}), "writing resource icon dir root entry")
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryEntry{
		ID:     ResourceGroupIcon,
		Offset: 0x80000000 | uint32(groupIconResDirOffset),
	}), "writing resource icon group dir root entry")

	// Icon resource directory
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryTable{
		NumIDEntries: 1,
		MajorVersion: 4,
	}), "writing icon resource dir")
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryEntry{
		ID:     1,
		Offset: 0x80000000 | uint32(iconResDir2Offset),
	}), "writing icon resource dir entry")

	// Icon resource directory 2
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryTable{
		NumIDEntries: 1,
		MajorVersion: 4,
	}), "writing icon resource dir 2")
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryEntry{
		ID:     1033,
		Offset: uint32(iconResDataEntryOffset),
	}), "writing icon resource dir entry 2")

	// Icon data entry
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDataEntry{
		DataRVA:  0x1000 + uint32(iconOffset),
		Size:     uint32(dib.size),
		Codepage: 1252,
	}), "writing icon data entry")

	// Group icon resources directory
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryTable{
		NumIDEntries: 1,
		MajorVersion: 4,
	}), "writing icon group resource dir")
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryEntry{
		ID:     1,
		Offset: 0x80000000 | uint32(groupIconResDir2Offset),
	}), "writing icon group resource dir entry")

	// Group icon resources directory 2
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryTable{
		NumIDEntries: 1,
		MajorVersion: 4,
	}), "writing group icon resource dir 2")
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDirectoryEntry{
		ID:     1033,
		Offset: uint32(groupIconDataEntryOffset),
	}), "writing group icon resource dir 2 entry")

	// Group icon data entry
	must(binary.Write(exeWriter, binary.LittleEndian, ResourceDataEntry{
		DataRVA:  0x1000 + uint32(groupIconOffset),
		Size:     uint32(groupIconSize),
		Codepage: 1252,
	}), "writing group icon data entry")

	w := io.MultiWriter(exeWriter, icoWriter)

	// Group icon directory
	must(binary.Write(w, binary.LittleEndian, GroupIconDirectory{
		Type:  1,
		Count: 1,
	}), "writing group icon directory")

	// Group icon directory entry
	must(binary.Write(exeWriter, binary.LittleEndian, GroupIconDirectoryEntry{
		Width:      uint8(dib.IconGroupWidth()),
		Height:     uint8(dib.IconGroupHeight()),
		ColorCount: uint8(dib.numColors),
		NumPlanes:  1,
		BPP:        uint16(dib.bpp),
		ImageSize:  uint32(dib.size),
		ResourceID: 1,
	}), "writing group icon directory entry (exe)")

	must(binary.Write(icoWriter, binary.LittleEndian, IconDirectoryEntry{
		Width:       uint8(dib.IconGroupWidth()),
		Height:      uint8(dib.IconGroupHeight()),
		ColorCount:  uint8(dib.numColors),
		NumPlanes:   1,
		ImageSize:   uint32(dib.size),
		ImageOffset: uint32(SizeOfGroupIconDirectory + SizeOfIconDirectoryEntry),
	}), "writing icon directory entry (ico)")

	dib.Write(w)
}

func must(err error, format string, args ...any) {
	if err != nil {
		log.Fatalf("%s: %v", fmt.Sprintf(format, args...), err)
	}
}
