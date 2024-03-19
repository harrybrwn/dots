package git

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const (
	// HashSize is the size hash for the current hash algorithm being used.
	HashSize = sha1.Size
	// MaxHashSize is the maximum length that of a git hash no matter what
	// algorithm is used.
	MaxHashSize = 32
)

const (
	mtimeChanged = 0x0001
	ctimeChanged = 0x0002
	ownerChanged = 0x0004
	modeChanged  = 0x0008
	inodeChanged = 0x0010
	dataChanged  = 0x0020
	typeChanged  = 0x0040
)

type index struct {
	header  indexCacheHeader
	entries []indexCacheEntry
}

func (ix *index) indexDiff(workingTree string) ([]*ModifiedFile, error) {
	mods := make([]*ModifiedFile, 0)
	for _, entry := range ix.entries {
		mod := ModifiedFile{
			Name: entry.name,
			Src: ObjModification{
				Mode: int(entry.mode),
				Hash: hex.EncodeToString(entry.oid),
			},
		}
		fname := filepath.Join(workingTree, entry.name)

		info, err := os.Lstat(fname)
		if err != nil {
			if os.IsNotExist(err) {
				mod.Type = ModDelete
				mod.Dst.Hash = string(make([]byte, hex.EncodedLen(HashSize))) // zero hash
				goto next
			} else {
				return nil, err
			}
		}

		if !info.ModTime().Equal(entry.statData.mtime.Time()) ||
			info.Size() != int64(entry.statData.size) ||
			info.Mode().Perm() != entry.mode.Perm() {
			mod.Type = ModChanged
		} else {
			switch sys := info.Sys().(type) {
			case *syscall.Stat_t:
				if uint64(sys.Dev) != uint64(entry.statData.dev) ||
					sys.Uid != entry.statData.uid ||
					sys.Gid != entry.statData.gid ||
					uint64(sys.Ino) != uint64(entry.statData.ino) {
					mod.Type = ModChanged
				} else {
					continue
				}
			default:
				continue
			}
		}
		mod.Dst.Mode = int(info.Mode())
		if mod.Dst.Mode&0100 != 0 {
			mod.Dst.Mode = 0644
		} else {
			mod.Dst.Mode = 0755
		}
		mod.Dst.Mode |= 0100000
		// mod.Dst.Mode = int(info.Mode())
	next:
		mods = append(mods, &mod)
	}
	return mods, nil
}

// look for `struct cache_header` in read-cache-ll.h
type indexCacheHeader struct {
	signature uint32
	version   uint32
	entries   uint32
}

func (hdr *indexCacheHeader) UnmarshalBinary(data []byte) error {
	const L = unsafe.Sizeof(indexCacheHeader{})
	const S = unsafe.Sizeof(uint32(0))
	if uintptr(len(data)) < L {
		return errors.New("to short to unmarshal binary")
	}
	hdr.signature = binary.BigEndian.Uint32(data[:L-(S*2)])
	hdr.version = binary.BigEndian.Uint32(data[L-(S*2):])
	hdr.entries = binary.BigEndian.Uint32(data[L-S:])
	if hdr.signature != cacheSignature {
		return errors.New("invalid index cache header signature")
	}
	return nil
}

func readIndex(r io.Reader) (*index, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	size := uint(len(raw))
	var ix index
	if err = ix.header.UnmarshalBinary(raw); err != nil {
		return nil, err
	}
	ix.entries = make([]indexCacheEntry, ix.header.entries)
	raw = raw[12:]
	size -= 12
	offset := uint(0)
	for i := uint32(0); offset < size && i < ix.header.entries; i++ {
		consumed, err := ix.entries[i].unmarshalBinary(raw[offset:], &ix.header)
		if err != nil {
			return nil, err
		}
		ix.entries[i].index = uint(i)
		offset += consumed
	}
	return &ix, nil
}

// from statinfo.h
type indexCacheTime struct{ sec, nsec uint32 }

func (ct *indexCacheTime) Time() time.Time {
	return time.Unix(int64(ct.sec), int64(ct.nsec))
}

// from statinfo.h
type indexStatData struct {
	ctime indexCacheTime
	mtime indexCacheTime
	dev   uint32
	ino   uint32
	uid   uint32
	gid   uint32
	size  uint32
}

// from read-cache-ll.h
type indexCacheEntry struct {
	statData indexStatData
	mode     fs.FileMode
	flags    uint
	//memPoolAlloc uint
	nameLen uint
	index   uint
	oid     []byte
	name    string
}

type indexOnDiskCacheEntry struct {
	ctime indexCacheTime
	mtime indexCacheTime
	dev   uint32
	ino   uint32
	mode  uint32
	uid   uint32
	gid   uint32
	size  uint32
	/**
	 * data struct {
	 *   hash  [hash_size]byte
	 *   flags uint16
	 *   if (flags & CE_EXTENDED)
	 *     flags2 uint16
	 * }
	 */
	data [MaxHashSize + 2*unsafe.Sizeof(uint16(0))]byte
	name string
}

const (
	cacheSignature = 0x44495243
	// provided by "read-cache.c"
	ceNameMask  = 0x0fff
	ceStageMask = 0x3000
	// provided by "read-cache-ll.h"
	// #define CE_EXTENDED 0x4000
	ceExtended = 0x4000
	ceValid    = 0x8000
	// provided by "read-cache-ll.h"
	// #define CE_EXTENDED_FLAGS (CE_INTENT_TO_ADD | CE_SKIP_WORKTREE)
	ceExtendedFlags    = uint((1 << 29) | (1 << 30))
	ceNotExtendedFlags = ^ceExtendedFlags
)

func (ce *indexCacheEntry) unmarshalBinary(data []byte, hdr *indexCacheHeader) (uint, error) {
	const offset = uint(unsafe.Offsetof(indexOnDiskCacheEntry{}.data))

	flagsp := data[offset+HashSize:]
	flags := uint(binary.BigEndian.Uint16(flagsp))
	length := flags & ceNameMask
	expandNameField := hdr.version == 4
	copyLength := uint(0)
	var name []byte

	if (flags & ceExtended) != 0 {
		extendedFlags := uint(binary.BigEndian.Uint16(flagsp)) << 16
		if (extendedFlags & (^ceExtendedFlags)) != 0 {
			panic(fmt.Sprintf("unknown index entry format 0x%08x", extendedFlags))
		}
		flags |= extendedFlags
		name = readCstringBytes(flagsp[2*unsafe.Sizeof(uint16(0)):])
	} else {
		name = readCstringBytes(flagsp[unsafe.Sizeof(uint16(0)):])
	}

	if expandNameField {
		panic("unfinished. Look for expand_name_field block in `create_from_disk` in read-cache.c")
	}

	if length == ceNameMask {
		length = uint(len(name))
		if expandNameField {
			length += copyLength
		}
	}

	ce.statData.ctime.sec = binary.BigEndian.Uint32(data)
	ce.statData.ctime.nsec = binary.BigEndian.Uint32(data[4:])
	ce.statData.mtime.sec = binary.BigEndian.Uint32(data[8:])
	ce.statData.mtime.nsec = binary.BigEndian.Uint32(data[12:])
	ce.statData.dev = binary.BigEndian.Uint32(data[16:])
	ce.statData.ino = binary.BigEndian.Uint32(data[20:])
	ce.mode = fs.FileMode(binary.BigEndian.Uint32(data[24:]))
	ce.statData.uid = binary.BigEndian.Uint32(data[28:])
	ce.statData.gid = binary.BigEndian.Uint32(data[32:])
	ce.statData.size = binary.BigEndian.Uint32(data[36:])

	ce.oid = data[offset : offset+HashSize]
	ce.flags = flags & (^uint(ceNameMask)) // remove the string length from flags
	ce.nameLen = length
	if expandNameField {
		panic("unfinished. Look for expand_name_field block in `create_from_disk` in read-cache.c")
		// if copyLength != 0 {
		// 	panic("unfinished. look for expand_name_field block in `create_from_disk` in read-cache.c")
		// }
		// ce.name = string(name[copyLength:])
	} else {
		ce.name = string(name)
		return cacheEntryDiskLength(ce), nil
	}
}

func cacheEntryDiskLength(ce *indexCacheEntry) uint {
	const hashSize = uint(HashSize)
	const uint16Size = uint(unsafe.Sizeof(uint16(0)))
	const dataOffset = uint(unsafe.Offsetof(indexOnDiskCacheEntry{}.data))
	var nflags uint
	if (ce.flags & ceExtended) != 0 {
		nflags = 2
	} else {
		nflags = 1
	}
	// alignment flex calculation (see read-cache.c)
	return (dataOffset + (hashSize + nflags*uint16Size + ce.nameLen) + 8) & (^uint(7))
}

func readCstringBytes(raw []byte) []byte {
	i := bytes.IndexByte(raw, 0)
	if i < 0 {
		return nil
	}
	return raw[:i]
}
