package git

import (
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/matryer/is"
)

func TestIndex(t *testing.T) {
	is := is.New(t)
	// Setup Index
	m := meta(t)
	err := setupTestRepo(
		m.Git(),
		newfile("test_index_file.txt", "this is a file"),
	)
	is.NoErr(err)
	is.NoErr(m.Git().Add("."))
	is.NoErr(m.Git().Commit("commit message"))
	is.NoErr(addFileToTree(m.Git(), newfile("second_file.txt", "this is the second file...")))
	is.NoErr(m.Git().Add("second_file.txt"))
	is.NoErr(m.Git().Commit("2nd commit"))
	// Read/Test index file
	f := must(os.Open(m.Git().indexFile()))
	index, err := readIndex(f)
	is.NoErr(err)
	is.NoErr(f.Close())
	is.True(len(index.entries) > 0)
	is.Equal(index.entries[0].name, "second_file.txt")
	is.Equal(index.entries[1].name, "test_index_file.txt")
	is.Equal(index.entries[1].statData.size, uint32(14))
	is.True(index.entries[1].mode.IsRegular())
	is.True(!index.entries[1].mode.IsDir())
	for _, e := range index.entries {
		is.True(len(e.name) > 0)
		is.Equal(e.nameLen, uint(len(e.name)))
		is.Equal(len(e.oid), 20)
		is.True(e.statData.ctime.sec > 0)
		is.True(e.statData.ctime.nsec > 0)
		is.True(e.statData.mtime.sec > 0)
		is.True(e.statData.mtime.nsec > 0)
		is.True(e.statData.dev != 0)
		is.True(e.statData.ino != 0)
		is.Equal(e.statData.gid, uint32(os.Getgid()))
		is.Equal(e.statData.uid, uint32(os.Getuid()))
		is.Equal(e.mode.Perm(), fs.FileMode(0644))
		oid := hex.EncodeToString(e.oid)
		objPath := filepath.Join(m.tmp, "repo/objects", oid[:2], oid[2:])
		is.True(exists(objPath))
	}
}
