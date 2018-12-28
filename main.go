package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func init() {
	log.SetOutput(os.Stdout)
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

// Branch ...
type Branch struct {
	Commit string
	Name   string
}

// Branches ...
type Branches []Branch

func reverseRevs(ss []Rev) {
	last := len(ss) - 1
	for i := 0; i < len(ss)/2; i++ {
		ss[i], ss[last-i] = ss[last-i], ss[i]
	}
}

func reverse(ss []Branch) {
	last := len(ss) - 1
	for i := 0; i < len(ss)/2; i++ {
		ss[i], ss[last-i] = ss[last-i], ss[i]
	}
}

func (b *BuildRoot) branchesByAge() Branches {
	cmd := exec.Command("git", "for-each-ref", "--sort=committerdate", "--format='%(objectname) %(refname)")
	cmd.Dir = b.BuildPath
	out, err := cmd.Output()
	if err != nil {
		log.Error("show ref failed", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	branches := Branches{}
	head, _ := regexp.Compile(".*/HEAD$")
	heads, _ := regexp.Compile("refs/heads/")
	re1, _ := regexp.Compile("\\^{}")
	re2, _ := regexp.Compile("[^/]*/[^/]*/")
	for scanner.Scan() {
		t := strings.Fields(scanner.Text())
		if head.MatchString(t[1]) {
			continue
		}
		if heads.MatchString(t[1]) {
			continue
		}
		n := re1.ReplaceAllString(t[1], "")
		n = re2.ReplaceAllString(n, "")
		if _, err := os.Stat(fmt.Sprintf("%s/ignore/%s", b.OutPath, t[0])); !os.IsNotExist(err) {
			continue
		}
		branches = append(branches, Branch{Commit: t[0], Name: n})
	}
	reverse(branches)
	return branches
}

func (b *BuildRoot) branches() Branches {
	cmd := exec.Command("git", "show-ref", "-d")
	cmd.Dir = b.BuildPath
	out, err := cmd.Output()
	if err != nil {
		log.Error("show ref failed")
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	branches := Branches{}
	head, _ := regexp.Compile(".*/HEAD$")
	heads, _ := regexp.Compile("refs/heads/")
	re1, _ := regexp.Compile("\\^{}")
	re2, _ := regexp.Compile("[^/]*/[^/]*/")
	for scanner.Scan() {
		t := strings.Fields(scanner.Text())
		if head.MatchString(t[1]) {
			continue
		}
		if heads.MatchString(t[1]) {
			continue
		}
		n := re1.ReplaceAllString(t[1], "")
		n = re2.ReplaceAllString(n, "")
		if _, err := os.Stat(fmt.Sprintf("%s/ignore/%s", b.OutPath, t[0])); !os.IsNotExist(err) {
			continue
		}
		branches = append(branches, Branch{Commit: t[0], Name: n})
	}
	reverse(branches)
	return branches
}

// Rev ...
type Rev struct {
	Commit  string
	Email   string
	Comment string
	State   string
}

// Revs ...
type Revs []Rev

func (b *BuildRoot) revlist(rev Branch) Revs {
	cmd := exec.Command("git", "rev-list", "--first-parent",
		"--pretty=format:%H %ce %s", string(rev.Name))
	cmd.Dir = b.BuildPath
	out, err := cmd.Output()
	if err != nil {
		log.Error("git rev-list failed")
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	r := Revs{}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "commit") {
			continue
		}
		t := strings.Fields(scanner.Text())
		commit := t[0]
		email := t[1]
		comment := strings.Join(t[2:], " ")
		if _, err := os.Stat(fmt.Sprintf("%s/ignore/%s", b.OutPath, commit)); !os.IsNotExist(err) {
			// never print ignored commits
			continue
		}
		r = append(r, Rev{Commit: commit, Email: email, Comment: comment})
		if _, err := os.Stat(fmt.Sprintf("%s/pass/%s", b.OutPath, commit)); !os.IsNotExist(err) {
			// return first passing commit
			return r
		}
	}
	return r
}

func (b *BuildRoot) nextrev(rev Branch) (string, error) {
	revs := b.revlist(rev)
	pass := ""
	fail := ""
	pending := ""
	last := ""
	for _, r := range revs {
		if _, err := os.Stat(fmt.Sprintf("%s/pass/%s", b.OutPath, r.Commit)); !os.IsNotExist(err) {
			pass = r.Commit
		} else if _, err := os.Stat(fmt.Sprintf("%s/fail/%s", b.OutPath, r.Commit)); !os.IsNotExist(err) {
			fail = r.Commit
		} else if pending == "" && fail == "" {
			pending = r.Commit
		}
		last = r.Commit
	}
	if pending != "" {
		return pending, nil
	}
	if fail != "" && pass != "" {
		t, _ := b.bisect(pass, fail)
		return t, nil
	}
	if fail != "" && last != fail && last != "" {
		t, _ := b.bisect(pass, fail)
		return t, nil
	}
	return "", nil
}

func (b *BuildRoot) bisect(pass, fail string) (string, error) {
	if pass == "" || fail == "" {
		return "", nil
	}
	out, err := exec.Command("git", "bisect", "--first-parent",
		"--bisect-all", fmt.Sprintf("%s^", fail), fmt.Sprintf("^%s", pass)).Output()
	if err != nil {
		log.Info("git bisect all failed", " pass=", pass, " fail=", fail)
		out, err = exec.Command("git", "bisect", "--first-parent",
			fmt.Sprintf("%s^", fail), fmt.Sprintf("^%s", pass)).Output()
		if err != nil {
			log.Info("git bisect failed", " pass=", pass, " fail=", fail)
		}
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		_pass := false
		_fail := false
		if _, err := os.Stat(fmt.Sprintf("%s/pass/%s", b.OutPath, line)); !os.IsNotExist(err) {
			_pass = true
		}
		if _, err := os.Stat(fmt.Sprintf("%s/fail/%s", b.OutPath, line)); !os.IsNotExist(err) {
			_fail = true
		}
		if _pass && _fail {
			return line, nil
		}
	}
	return "", nil
}

func (b *BuildRoot) buildSetup(commit string) {
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

func (b *BuildRoot) runBuild(commit string) {
	r, err := git.PlainOpen(b.BuildPath)
	if err != nil {
		log.Fatalf("err %v", err)
	}
	w, err := r.Worktree()
	if err != nil {
		log.Fatalf("err %v", err)
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
		log.Fatal(err)
	}

	_, err = f.Write(stdoutStderr)
	if err != nil {
		log.Fatal(err)
	}

	if rc == 0 {
		err := os.Rename("log.out", fmt.Sprintf("%s/pass/%s", b.OutPath, commit))
		if err != nil {
			log.Fatal(err)
		}
	} else {
		err := os.Rename("log.out", fmt.Sprintf("%s/fail/%s", b.OutPath, commit))
		if err != nil {
			log.Fatal(err)
		}
	}
}

// BuildRoot ...
type BuildRoot struct {
	BuildPath   string `json:"build_path"`
	OutPath     string `json:"out_path"`
	Repo        string `json:"repo"`
	BuildScript string `json:"build_script"`
}

func main() {
	dat, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	b := BuildRoot{}
	err = json.Unmarshal(dat, &b)
	if err != nil {
		log.Fatal(err)
	}

	_, err = git.PlainClone(b.BuildPath, false, &git.CloneOptions{
		URL:               b.Repo,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	})
	if err != nil {
		log.Warn(err)
	}

	for _, branch := range b.branches() {
		_nextrev, err := b.nextrev(branch)
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

		b.buildSetup(_nextrev)
		b.runBuild(_nextrev)
	}
	b.report()
}

func (b *BuildRoot) report() {
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
