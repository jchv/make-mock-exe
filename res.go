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
	SizeOfGroupIconDirectory      = 6
	SizeOfGroupIconDirectoryEntry = 14
)

// GroupIconDirectory is the data structure pointed to by ResourceGroupIcon
// resource data entries. It is followed by Count instances of
// GroupIconDirectoryEntry.
type GroupIconDirectory struct {
	Reserved uint16
	Type     uint16
	Count    uint16
}

// GroupIconDirectoryEntry are entries of the GroupIconDirectory structure.
type GroupIconDirectoryEntry struct {
	Width      uint8 // 0 if >=256
	Height     uint8 // 0 if >=256
	ColorCount uint8
	Reserved   uint8
	NumPlanes  uint16
	BPP        uint16
	ImageSize  uint32
	ResourceID uint16
}
