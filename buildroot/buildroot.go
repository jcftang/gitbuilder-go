package buildroot

import (
	"fmt"
	"os"

	"github.com/jcftang/gitbuilder-go/buildroot/git"
	"github.com/jcftang/gitbuilder-go/repo"
	log "github.com/sirupsen/logrus"
)

// BuildRoot ...
type BuildRoot interface {
	RunSetup(commit string)
	RunBuild(commit string) error
	RunReport()
	Branches() repo.Branches
	BranchesByAge() repo.Branches
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

func isPass(commit, outPath string) bool {
	if _, err := os.Stat(fmt.Sprintf("%s/pass/%s", outPath, commit)); !os.IsNotExist(err) {
		return true
	}
	return false
}

func isFail(commit, outPath string) bool {
	if _, err := os.Stat(fmt.Sprintf("%s/fail/%s", outPath, commit)); !os.IsNotExist(err) {
		return true
	}
	return false
}

// RunAll Executes the repo setup, build/test and report
func RunAll(b BuildRoot, c Config) error {
	for _, branch := range b.Branches() {
		_nextrev, err := b.NextRev(branch)
		if err != nil {
			log.Error(err)
		}
		if _nextrev == "" {
			log.Info("branch ", branch.Name, " is up to date")
			continue
		}
		if isPass(_nextrev, c.OutPath) {
			continue
		}
		if isFail(_nextrev, c.OutPath) {
			continue
		}

		b.RunSetup(_nextrev)
		err = b.RunBuild(_nextrev)
		if err != nil {
			log.Error(err)
		}
	}
	b.RunReport()
	return nil
}
