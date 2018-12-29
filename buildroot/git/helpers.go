package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Branch ...
type Branch struct {
	Commit string
	Name   string
}

// Branches ...
type Branches []Branch

// Rev ...
type Rev struct {
	Commit  string
	Email   string
	Comment string
	State   string
}

// Revs ...
type Revs []Rev

func (b *Repo) revlist(rev Branch) Revs {
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

func (b *Repo) branches() Branches {
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

func (b *Repo) branchesByAge() Branches {
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

func (b *Repo) nextRev(rev Branch) (string, error) {
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

func (b *Repo) bisect(pass, fail string) (string, error) {
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
