package runner

import (
	"github.com/jcftang/gitbuilder-go/buildroot"
	log "github.com/sirupsen/logrus"
)

// RunAll Executes the repo setup, build/test and report
func RunAll(b buildroot.BuildRoot) error {
	for _, branch := range b.Branches() {
		_nextrev, err := b.NextRev(branch)
		if err != nil {
			log.Error(err)
		}
		if _nextrev == "" {
			log.Info("branch ", branch.Name, " is up to date")
			continue
		}
		if b.IsPass(_nextrev) {
			continue
		}
		if b.IsFail(_nextrev) {
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
