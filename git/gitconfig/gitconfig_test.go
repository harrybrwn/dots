package gitconfig

import (
	"testing"

	"github.com/matryer/is"
)

func TestConfig(t *testing.T) {
	config := `[core]
	repositoryformatversion = 0
	fileMode = true
	bare = false
	logallrefupdates = true
[remote "origin"]
	url = git@github.com:harrybrwn/dots.git
	fetch = +refs/heads/*:refs/remotes/origin/*`
	is := is.New(t)
	p := &configParser{[]byte(config), 1, false}
	// c, _, err := Parse([]byte(config))
	c, err := p.parse()
	is.NoErr(err)
	is.Equal(c.sections["core"].entries["bare"], "false")
	is.Equal(c.sections["core"].entries["repositoryformatversion"], "0")
	is.Equal(c.sections["core"].entries["filemode"], "true")
	is.Equal(c.sections["core"].entries["logallrefupdates"], "true")
	is.Equal(
		c.sections["remote"].subsections["origin"].entries["url"],
		"git@github.com:harrybrwn/dots.git",
	)
	is.Equal(
		c.sections["remote"].subsections["origin"].entries["fetch"],
		"+refs/heads/*:refs/remotes/origin/*",
	)
}

func TestDumbParse(t *testing.T) {
	config := `[core]
	repositoryformatversion = 0
	fileMode = true
	bare = false
	logallrefupdates = true
[remote "origin"]
	url = git@github.com:harrybrwn/dots.git
	fetch = +refs/heads/*:refs/remotes/origin/*`
	is := is.New(t)
	p := &configParser{[]byte(config), 1, false}
	c, err := p.dumbParse()
	is.NoErr(err)
	is.Equal(c["remote.origin.url"], "git@github.com:harrybrwn/dots.git")
	is.Equal(c["remote.origin.fetch"], "+refs/heads/*:refs/remotes/origin/*")
	is.Equal(c["core.filemode"], "true")
	is.Equal(c["core.bare"], "false")
	is.Equal(c["core.repositoryformatversion"], "0")
}

func TestParseSection(t *testing.T) {
	is := is.New(t)
	var p *configParser
	p = &configParser{[]byte(`remote "origin"]`), 1, false}
	k, err := p.getSectionKey()
	is.NoErr(err)
	is.Equal(k, "remote.origin")
	p = &configParser{[]byte("remote \t \"origin\"]"), 1, false}
	name, sub, err := p.getSectionFullName()
	is.NoErr(err)
	is.Equal(name, "remote")
	is.Equal(sub, "origin")
}
