package git

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strconv"
	"strings"
	"time"
)

type Object struct {
	Type ObjectType
	Size uint64
	Data []byte
	Hash string
}

func (o *Object) writeTo(w io.Writer) (written int, err error) {
	var n int
	n, err = fmt.Fprintf(w, "%s %d", o.Type.String(), o.Size)
	if err != nil {
		return
	}
	written += n
	n, err = w.Write([]byte{0})
	if err != nil {
		return
	}
	written += n
	n, err = w.Write(o.Data)
	if err != nil {
		return
	}
	written += n
	return written, err
}

type FileObject struct {
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

func NewObjectFromFile(file fs.File) (*FileObject, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()
	t := ObjBlob
	hash := objectHash(t, uint64(size), file)
	return &FileObject{
		Name: stat.Name(),
		Type: t,
		Size: size,
		Hash: hex.EncodeToString(hash),
	}, nil
}

type Commit struct {
	Tree         [HashSize]byte
	Parent       [HashSize]byte
	Author       string
	AuthorTime   time.Time
	Commiter     string
	CommiterTime time.Time
	Message      string
}

func (c *Commit) IsRoot() bool {
	for i := 0; i < HashSize; i++ {
		if c.Parent[i] != 0 {
			return false
		}
	}
	return true
}

const TreeMode = fs.FileMode(040000)

type TreeEntry struct {
	Mode fs.FileMode
	Name string
	Hash [HashSize]byte
}

type LogFlag uint

const (
	LogFlagInitial LogFlag = iota
	LogFlagAmend
)

type Log struct {
	Hash      [HashSize]byte
	Prev      [HashSize]byte
	Author    string
	TimeStamp time.Time
	Message   string
	Flag      LogFlag
}

func objectHash(typ ObjectType, size uint64, r io.Reader) []byte {
	h := sha1.New()
	h.Write([]byte(typ.String()))
	h.Write([]byte{' '})
	h.Write([]byte(strconv.FormatUint(size, 10)))
	h.Write([]byte{0})
	_, _ = io.Copy(h, r)
	return h.Sum(nil)
}

func parseObject(r io.Reader, dst *Object) error {
	var (
		buf *bufio.Reader
		ok  bool
	)
	if buf, ok = r.(*bufio.Reader); !ok {
		buf = bufio.NewReader(r)
	}
	header, err := buf.ReadSlice(0)
	if err != nil {
		return err
	}
	if header[len(header)-1] == 0 {
		header = header[:len(header)-1]
	}
	head := strings.SplitN(string(header), " ", 2)
	if len(head) < 2 {
		return fmt.Errorf("invalid object header: %q", string(header))
	}
	dst.Type = objectType(head[0])
	dst.Size, err = strconv.ParseUint(head[1], 10, 64)
	if err != nil {
		return err
	}
	dst.Data, err = io.ReadAll(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func parseCommit(r io.Reader, dst *Commit) (err error) {
	var (
		buf *bufio.Reader
		ok  bool
	)
	if buf, ok = r.(*bufio.Reader); !ok {
		buf = bufio.NewReader(r)
	}
loop:
	for {
		line, err := buf.ReadBytes('\n')
		if err != nil {
			return err
		}
		if line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		parts := bytes.SplitN(line, []byte{' '}, 2)
		switch len(parts) {
		case 0:
			continue
		case 1:
			if len(parts[0]) == 0 {
				// this is the commit message
				break loop
			}
		}
		switch string(parts[0]) {
		case "tree":
			_, err = hex.Decode(dst.Tree[:], parts[1])
		case "parent":
			_, err = hex.Decode(dst.Parent[:], parts[1])
		case "author":
			dst.Author, dst.AuthorTime, err = parseCommitAuthor(parts[1])
		case "committer":
			dst.Commiter, dst.CommiterTime, err = parseCommitAuthor(parts[1])
		}
		if err != nil {
			return err
		}
	}
	all, err := io.ReadAll(buf)
	if err != nil {
		return err
	}
	dst.Message = string(bytes.TrimRight(all, "\n"))
	return nil
}

func parseTree(raw []byte) ([]TreeEntry, error) {
	entries := make([]TreeEntry, 0)
	start := 0
	l := len(raw)
	for i := bytes.IndexByte(raw, 0); i > 0 && i < l && start < l; {
		parts := bytes.SplitN(raw[start:i], []byte{' '}, 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid tree entry header")
		}
		i++
		mode, err := strconv.ParseUint(string(parts[0]), 8, 32)
		if err != nil {
			return nil, err
		}
		entry := TreeEntry{
			Mode: fs.FileMode(mode),
			Name: string(parts[1]),
		}
		if i >= l {
			break
		}
		start = i + HashSize
		copy(entry.Hash[:], raw[i:i+HashSize])
		i = start + bytes.IndexByte(raw[start:], 0)
		entries = append(entries, entry)
	}
	return entries, nil
}

func parseLogs(r io.Reader) ([]Log, error) {
	buf := bufio.NewReader(r)
	logs := make([]Log, 0)
	running := true
	for running {
		line, err := buf.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				running = false
			} else {
				return nil, err
			}
		}
		if len(line) <= 1 {
			continue
		}
		if line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		var log Log
		ix := bytes.IndexByte(line, '\t')
		if ix < 0 {
			continue
		}
		log.Message = string(line[ix:])
		parts := bytes.SplitN(line[:ix], []byte{' '}, 3)
		if len(parts) < 3 {
			continue
		}
		copy(log.Prev[:], parts[0])
		copy(log.Hash[:], parts[1])
		logs = append(logs, log)
	}
	return logs, nil
}

func parseCommitAuthor(line []byte) (author string, ts time.Time, err error) {
	locix := bytes.LastIndexByte(line, ' ')
	if locix < 0 {
		return "", ts, errors.New("invalid commit author timestamp")
	}
	tzoffset, err := parseTimeOffset(line[locix+1:])
	if err != nil {
		return "", ts, err
	}
	loc := time.FixedZone("", tzoffset)
	tsix := bytes.LastIndexByte(line[:locix], ' ')
	if tsix < 0 {
		return "", ts, errors.New("invalid commit author timestamp")
	}
	tsix++ // skip the space
	secs, err := strconv.ParseInt(string(line[tsix:locix]), 10, 64)
	if err != nil {
		return "", ts, err
	}
	ts = time.Unix(secs, 0)
	return string(line[:tsix]), ts.In(loc), nil
}

func parseTimeOffset(b []byte) (int, error) {
	sign := 1
	if len(b) < 5 {
		return 0, errors.New("invalid timezone offset")
	}
	if b[0] == '-' {
		sign = -1
	} else if b[0] != '+' {
		return 0, errors.New("invalid timezone offset")
	}
	h := (int(b[1]-'0') * 10) + int(b[2]-'0')
	s := (int(b[3]-'0') * 10) + int(b[4]-'0')
	return sign * ((h * 60) + s) * 60, nil
}
