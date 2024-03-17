package git

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type Ref string

func NewHashRef(b [HashSize]byte) Ref {
	return Ref(hex.EncodeToString(b[:]))
}

func (ref Ref) IsHash() bool {
	dec, err := hex.DecodeString(string(ref))
	if err == nil && len(dec) == HashSize {
		return true
	}
	return false
}

func (ref Ref) Follow(g *Git) (Ref, error) {
	if ref.IsHash() {
		return "", errors.New("not a followable ref")
	}
	return readRef(filepath.Join(g.gitDir, string(ref)))
}

func (ref Ref) fullFollow(g *Git) (r Ref, err error) {
	r = ref
	if r.IsHash() {
		return r, nil
	}
	for !r.IsHash() {
		r, err = readRef(filepath.Join(g.gitDir, string(r)))
		if err != nil {
			return "", err
		}
	}
	return r, nil
}

func readRef(filename string) (Ref, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	all, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	ix := bytes.Index(all, []byte("ref:"))
	if ix >= 0 && len(all) > 4 {
		all = all[ix+4:]
	}
	all = bytes.Trim(all, " \t\r\n")
	return Ref(all), nil
}
