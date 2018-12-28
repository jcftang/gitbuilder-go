# gitbuilder-go

This is a re-imagining of https://github.com/apenwarr/gitbuilder
but done in golang.

This works just enough for my own uses for building a project,
running tests and print a report of where things succeeded or failed.

To use this, something like the following can be done, you will
need golang 1.11 or newer

    git clone https://github.com/jcftang/gitbuilder-go.git
    cd gitbuilder-go
    go build
    ./gitbuilder-go config.json

Where config.json contains

    {
      "repo": "git://github.com/apenwarr/builder-test.git",
      "build_path": "/Users/jtang/develop/src/github.com/jcftang/gitbuilder/build",
      "out_path" : "/Users/jtang/develop/src/github.com/jcftang/gitbuilder/out",
      "build_script" : "/Users/jtang/develop/src/github.com/jcftang/gitbuilder/build.sh"
    }

Where 

* `repo` is the git repo url to clone 
* `build_path` is the directory location where the repo will be cloned to
* `out_path` is the directory where the logs are dumped to
* `build_script` is an executable that is used to *build* the project, this is up to the user to create and populate

# Why attempt to rewrite gitbuilder

* I wanted an app that is a bit more flexible that the shell/perl/cgi implementation had
* I wanted to simply the setup process a bit and make it easier to add and update things with
* I don't remember perl very well and shell scripts while they work aren't that nice to maintain
* I wanted something that worked on my mac as well as linux
* I wanted something small, hackable something closer to what buildbot is like

# TODO's

* Add a web ui like the original
* Check for warnings
* Add a deadline to the build command itself to prevent long running jobs
* Put the bisect/nextrev stuff behind an interface
