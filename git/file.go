package git

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"io/fs"
	"strconv"
)

type Object struct {
	Name     string
	Modified bool
	Mode     int
	Type     ObjectType
	Hash     string
	Size     int64
}

type ObjectType uint8

const (
	ObjBlob ObjectType = iota
	ObjTree
	ObjCommit
	ObjTag
	ObjUnknown
)

func (ot ObjectType) String() string {
	switch ot {
	case ObjBlob:
		return "blob"
	case ObjTree:
		return "tree"
	case ObjCommit:
		return "commit"
	case ObjTag:
		return "tag"
	default:
		return "unknown"
	}
}

func objectType(s string) ObjectType {
	switch s {
	case "blob":
		return ObjBlob
	case "tree":
		return ObjTree
	case "commit":
		return ObjCommit
	case "tag":
		return ObjTag
	default:
		return ObjUnknown
	}
}

func NewObjectFromFile(file fs.File) (*Object, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	t := ObjBlob
	return &Object{
		Name: stat.Name(),
		Type: t,
		Size: stat.Size(),
		Hash: objectHash(t, stat.Size(), file),
	}, nil
}

func objectHash(typ ObjectType, size int64, r io.Reader) string {
	h := sha1.New()
	h.Write([]byte(typ.String()))
	h.Write([]byte{' '})
	h.Write([]byte(strconv.FormatInt(size, 10)))
	h.Write([]byte{0})
	io.Copy(h, r)
	return hex.EncodeToString(h.Sum(nil))
}
