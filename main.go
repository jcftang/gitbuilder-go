package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/jcftang/gitbuilder-go/buildroot"

	log "github.com/sirupsen/logrus"
	git "gopkg.in/src-d/go-git.v4"
)

func init() {
	log.SetOutput(os.Stdout)
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

	br := buildroot.New(buildroot.Config{
		BuildPath:   b.BuildPath,
		OutPath:     b.OutPath,
		Repo:        b.Repo,
		BuildScript: b.BuildScript,
	})
	br.RunAll()
}
