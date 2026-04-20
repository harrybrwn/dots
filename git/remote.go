package git

import (
	"os"
	"path/filepath"
)

func (g *Git) CreateRemoteRef(remoteName, branchName string, ref Ref) error {
	remoteDir := filepath.Join(g.gitDir, "refs/remotes", remoteName)
	filename := filepath.Join(remoteDir, branchName)
	err := os.MkdirAll(remoteDir, os.FileMode(0775))
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0664)
	if err != nil {
		return err
	}
	defer f.Close()
	if !ref.IsHash() {
		_, err = f.WriteString("ref: ")
		if err != nil {
			return err
		}
	}
	_, err = f.WriteString(string(ref))
	if err != nil {
		return err
	}
	return nil
}
