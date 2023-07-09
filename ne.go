// make-mock-exe by John Chadwick <john@jchw.io>
//
// To the extent possible under law, the person who associated CC0 with
// make-mock-exe has waived all copyright and related or neighboring rights
// to make-mock-exe.
//
// You should have received a copy of the CC0 legalcode along with this
// work.  If not, see <http://creativecommons.org/publicdomain/zero/1.0/>.

package main

const (
	SizeOfNEFileHeader          = 64
	SizeOfNESegment             = 8
	SizeOfNEResourceTableHeader = 2
	SizeOfNEResourceTableEntry  = 8
	SizeOfNEResource            = 12
)

type NEFileHeader struct {
	Signature                    [2]byte
	MajorLinkerVersion           byte
	MinorLinkerVersion           byte
	EntryTableOffset             uint16
	EntryTableLength             uint16
	FileLoadCRC                  uint32
	Flag                         uint16
	AutoDataSegmentIndex         uint16
	InitialHeap                  uint16
	InitialStack                 uint16
	Entrypoint                   uint32
	InitStack                    uint32
	NumberOfSegments             uint16
	NumberOfModuleReferences     uint16
	NonResidentNameTableSize     uint16
	OffsetOfSegmentTable         uint16
	OffsetOfResourceTable        uint16
	OffsetOfResidentNameTable    uint16
	OffsetOfModuleReferenceTable uint16
	OffsetOfImportedNamesTable   uint16
	OffsetOfNonResidentNameTable uint32
	NumberOfMovableEntries       uint16
	FileAlignmentShiftCount      uint16
	NumberOfResourceEntries      uint16
	ExecutableType               uint8
	Reserved                     [9]byte
}

type NESegment struct {
	LogicalSectorOffset uint16
	SizeOnDisk          uint16
	Flag                uint16
	TotalSize           uint16
}

type NEResourceTableHeader struct {
	AlignmentShiftCount uint16
}

type NEResourceTableEntry struct {
	TypeID       uint16
	NumResources uint16
	Resource     [2]uint16
}

type NEResource struct {
	DataOffsetShifted uint16
	DataLength        uint16
	Flags             uint16
	ResourceID        uint16
	Resource          [2]uint16
}
