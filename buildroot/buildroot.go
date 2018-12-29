package buildroot

import (
	"github.com/jcftang/gitbuilder-go/buildroot/git"
	"github.com/jcftang/gitbuilder-go/repo"
)

// BuildRoot ...
type BuildRoot interface {
	RunSetup(commit string)
	RunBuild(commit string) error
	RunReport()
	RunAll() error
	Branches() repo.Branches
	NextRev(branch repo.Branch) (string, error)
}

// Config ...
type Config struct {
	BuildPath   string `json:"build_path"`
	OutPath     string `json:"out_path"`
	Repo        string `json:"repo"`
	BuildScript string `json:"build_script"`
}

// New Buildroot instance
func New(b Config) BuildRoot {
	return git.New(b.BuildPath, b.OutPath, b.Repo, b.BuildScript)
}
