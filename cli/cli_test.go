package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestClone(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	cmd := NewRootCmd()
	err := cmd.ParseFlags([]string{
		"-d", filepath.Join(tmp, "tree"),
		"-c", tmp,
	})
	if err != nil {
		t.Fatal(err)
	}
	c, args, err := cmd.Find([]string{
		"clone",
		"https://github.com/harrybrwn/utest",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = c.RunE(c, args)
	if err != nil {
		t.Fatal(err)
	}
	ex := exec.Command("exa", "--tree", tmp)
	ex.Stdout = os.Stdout
	ex.Run()
	_, err = os.Stat(filepath.Join(tmp, repo))
	if os.IsNotExist(err) {
		t.Error("headless repo should exist")
	}
}
