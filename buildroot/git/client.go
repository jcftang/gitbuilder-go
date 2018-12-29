package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// Repo ...
type Repo struct {
	BuildPath   string `json:"build_path"`
	OutPath     string `json:"out_path"`
	Repo        string `json:"repo"`
	BuildScript string `json:"build_script"`
}

// New git repo instance
func New(buildpath, outpath, repo, buildscript string) *Repo {
	return &Repo{
		BuildPath:   buildpath,
		OutPath:     outpath,
		Repo:        repo,
		BuildScript: buildscript,
	}
}

type exitStatus interface {
	ExitStatus() int
}

// ExitStatus returns the exit status of the error if it is an exec.ExitError
// or if it implements ExitStatus() int.
// 0 if it is nil or 1 if it is a different error.
func ExitStatus(err error) int {
	if err == nil {
		return 0
	}
	if e, ok := err.(exitStatus); ok {
		return e.ExitStatus()
	}
	if e, ok := err.(*exec.ExitError); ok {
		if ex, ok := e.Sys().(exitStatus); ok {
			return ex.ExitStatus()
		}
	}
	return 1
}

// RunAll ...
func (b *Repo) RunAll() error {
	for _, branch := range b.branches() {
		_nextrev, err := b.nextRev(branch)
		if err != nil {
			log.Error(err)
		}
		if _nextrev == "" {
			log.Info("branch ", branch.Name, " is up to date")
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("%s/pass/%s", b.OutPath, _nextrev)); !os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("%s/fail/%s", b.OutPath, _nextrev)); !os.IsNotExist(err) {
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

// RunSetup ...
func (b *Repo) RunSetup(commit string) {
	paths := []string{
		"pass",
		"ignore",
		"fail",
	}
	for _, p := range paths {
		opath := filepath.Join(b.OutPath, p)
		err := os.MkdirAll(opath, 0744)
		if err != nil {
			log.Fatalf("MkdirAll %q: %s", opath, err)
		}
	}

	c := []string{
		"git remote show | xargs git remote prune",
		"git remote update",
	}
	for _, i := range c {
		args := strings.Fields(i)
		cmd := exec.Command(args[0], args[1])
		cmd.Dir = b.BuildPath
		_, err := cmd.Output()
		rc := ExitStatus(err)
		if rc != 0 {
			log.Info("Exit code: ", rc, " cmd: ", i)
		}
	}
}

// RunBuild ...
func (b *Repo) RunBuild(commit string) error {
	r, err := git.PlainOpen(b.BuildPath)
	if err != nil {
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	hash := plumbing.NewHash(commit)
	checkoutopts := git.CheckoutOptions{Hash: hash}
	w.Checkout(&checkoutopts)

	cleanopts := git.CleanOptions{Dir: true}
	w.Clean(&cleanopts)

	resetopts := git.ResetOptions{Commit: hash, Mode: git.HardReset}
	w.Reset(&resetopts)

	//args := strings.Fields(fmt.Sprintf("../build.sh 2>&1"))
	args := strings.Fields(fmt.Sprintf("%s 2>&1", b.BuildScript))
	cmd := exec.Command(args[0], args[1])
	cmd.Dir = b.BuildPath
	stdoutStderr, err := cmd.CombinedOutput()
	rc := ExitStatus(err)
	if rc != 0 {
		log.Info("Non-zero exit code", rc)
	}

	f, err := os.Create("log.out")
	if err != nil {
		return err
	}

	_, err = f.Write(stdoutStderr)
	if err != nil {
		return err
	}
	if rc == 0 {
		err := os.Rename("log.out", fmt.Sprintf("%s/pass/%s", b.OutPath, commit))
		if err != nil {
			return err
		}
	} else {
		err := os.Rename("log.out", fmt.Sprintf("%s/fail/%s", b.OutPath, commit))
		if err != nil {
			return err
		}
	}
	return nil
}

// RunReport ...
func (b *Repo) RunReport() {
	// git for-each-ref --sort=-committerdate refs/
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Branch", "Status", "Commit", "Who", "Reason"})
	for _, branch := range b.branchesByAge() {
		revs := b.revlist(branch)
		fail := ""
		pending := ""
		for _, rev := range revs {
			commit := rev.Commit[:7]
			if _, err := os.Stat(fmt.Sprintf("%s/pass/%s", b.OutPath, rev.Commit)); !os.IsNotExist(err) {
				table.Append([]string{branch.Name, "ok", commit, rev.Email, rev.Comment})
				break
			}
			if _, err := os.Stat(fmt.Sprintf("%s/fail/%s", b.OutPath, rev.Commit)); !os.IsNotExist(err) {
				fail = rev.Commit
				table.Append([]string{branch.Name, "FAIL", commit, rev.Email, rev.Comment})
			}
			if pending == "" && fail == "" {
				pending = rev.Commit
			}
			if fail == "" && pending != "" {
				table.Append([]string{branch.Name, "pending", commit, rev.Email, rev.Comment})
				break
			}

		}
	}
	//table.SetAutoMergeCells(true)
	//table.SetRowLine(true)
	table.Render()
}
