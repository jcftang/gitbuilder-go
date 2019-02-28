package git

import (
	"context"
	"errors"
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

// Client ...
type Client struct {
	BuildPath   string `json:"build_path"`
	OutPath     string `json:"out_path"`
	Repo        string `json:"repo"`
	BuildScript string `json:"build_script"`
	logfile     *os.File
}

// New git repo instance
func New(buildpath, outpath, repo, buildscript string) *Client {
	return &Client{
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

// IsPass ...
func (b *Client) IsPass(commit string) bool {
	if _, err := os.Stat(fmt.Sprintf("%s/pass/%s", b.OutPath, commit)); !os.IsNotExist(err) {
		return true
	}
	return false
}

// IsFail ...
func (b *Client) IsFail(commit string) bool {
	if _, err := os.Stat(fmt.Sprintf("%s/fail/%s", b.OutPath, commit)); !os.IsNotExist(err) {
		return true
	}
	return false
}

// RunSetup ...
func (b *Client) RunSetup(ctx context.Context, commit string) {
	err := errors.New("Could not create log.out file")
	b.logfile, err = os.Create("log.out")
	if err != nil {
		log.Error(err)
	}
	b.logfile.WriteString("Running setup\n\n")
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
	b.logfile.WriteString("Updating repo\n\n")
	c := []string{
		"git remote show | xargs git remote prune",
		"git remote update",
	}
	for _, i := range c {
		args := strings.Fields(i)
		cmd := exec.CommandContext(ctx, args[0], args[1])
		cmd.Dir = b.BuildPath
		_, err := cmd.Output()
		rc := ExitStatus(err)
		if rc != 0 {
			log.Info("Exit code: ", rc, " cmd: ", i)
		}
	}
}

// RunBuild ...
func (b *Client) RunBuild(ctx context.Context, commit string) error {
	b.logfile.WriteString("Starting build\n\n")

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
	cmd := exec.CommandContext(ctx, args[0], args[1])
	cmd.Dir = b.BuildPath
	stdoutStderr, err := cmd.CombinedOutput()
	rc := ExitStatus(err)
	if rc != 0 {
		log.Info("Non-zero exit code", rc)
	}

	_, err = b.logfile.Write(stdoutStderr)
	if err != nil {
		return err
	}
	b.logfile.Close()
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
func (b *Client) RunReport() {
	// git for-each-ref --sort=-committerdate refs/
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Branch", "Status", "Commit", "Who", "Reason"})
	for _, branch := range b.BranchesByAge() {
		revs := b.revlist(branch)
		fail := ""
		pending := ""
		for _, rev := range revs {
			commit := rev.Commit[:7]
			if b.IsPass(rev.Commit) {
				table.Append([]string{branch.Name, "ok", commit, rev.Email, rev.Comment})
				break
			}
			if b.IsFail(rev.Commit) {
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
