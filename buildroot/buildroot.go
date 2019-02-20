package buildroot

import (
	"context"

	"github.com/jcftang/gitbuilder-go/buildroot/git"
	"github.com/jcftang/gitbuilder-go/repo"
)

// BuildRoot ...
type BuildRoot interface {
	RunSetup(ctx context.Context, commit string)
	RunBuild(ctx context.Context, commit string) error
	RunReport()
	Branches() repo.Branches
	BranchesByAge() repo.Branches
	NextRev(branch repo.Branch) (string, error)
	IsPass(commit string) bool
	IsFail(commit string) bool
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
