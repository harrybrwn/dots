package dotfiles

import "github.com/harrybrwn/dots/git"

type Repo interface {
	Git() *git.Git
}

type ReadmeFlag interface {
	HasReadme() bool
}
