package buildroot

import (
	"github.com/jcftang/gitbuilder-go/buildroot/git"
)

// BuildRoot ...
type BuildRoot interface {
	RunSetup(commit string)
	RunBuild(commit string)
	RunReport()
	RunAll()
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
