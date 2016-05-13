package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/docopt/docopt-go"
)

const (
	doc = `
Merge pull requests, do review, run tests.

Usage:
  git-mergetron <subcommand> [<args>...]

Subcommands:
`
	version = `git-mergetron 0.0.1`
)

type CmdArgs map[string]interface{}

type Cmd struct {
	desc string

	doc    string
	action func(CmdArgs)
}

var subcommands = map[string]Cmd{
	"merge": Cmd{"begin a manual merge of branches", mergeDoc, merge},
}

func main() {
	sublist := ""
	for k, v := range subcommands {
		sublist = sublist + fmt.Sprintf("\n  %s:   %s", k, v.desc)
	}
	args, err := docopt.Parse(doc+sublist, nil, true, version, true)
	if err != nil {
		log.Fatal(err)
	}

	subcmd := args["<subcommand>"].(string)

	if cmd, ok := subcommands[subcmd]; ok {
		subargs := args["<args>"].([]string)
		argv := make([]string, 0, len(subargs)+1)
		argv = append(argv, subcmd)
		argv = append(argv, subargs...)
		parsed, err := docopt.Parse(cmd.doc, argv, true, "", false)
		if err != nil {
			log.Fatal(err)
		}

		cmd.action(parsed)
	}

}

func orFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

var mergeDoc = cleanWS(`
	Usage:
	  git-mergetron merge <branches>...
	`)

func merge(args CmdArgs) {
	branches := args["<branches>"].([]string)

	orFatal(chDirRepoRoot())
	orFatal(makeSavepoint())
	orFatal(recordBranches(branches))
	orFatal(mergeBranches(branches))
	orFatal(spawnDifftool())
	orFatal(runQA())
}

var reviewDoc = cleanWS(`
  Usage:
	  git-mergetron review
`)

func review(args CmdArgs) {
	orFatal(chDirRepoRoot())
	orFatal(spawnDifftool())
	orFatal(runQA())
}

/*
git branch -d savepoint

rm -f .git/merges.sh

git push
git-clean-branches
*/
var completeDoc = cleanWS(`
  Useage:
	  git-mergetron complete
`)

func complete(args CmdArgs) {
	orFatal(chDirRepoRoot())
	orFatal(cleanRecordedBranches())
	orFatal(deleteSavePoint())
	orFatal(pushCurrent())
	orFatal(cleanBranches())
}

const branchRecordPath = ".git/mergetron-branches"

func cleanRecordedBranches() error {
	return os.Remove(branchRecordPath)
}

func deleteSavePoint() error {
	dsp := git("branch", "-d", "savepoint")
	return dsp.err
}

func pushCurrent() error {
	pc := git("push", "-u")
	return pc.err
}

/*
#!/usr/bin/env bash

remote=${1:-origin}

declare -a branches
declare -a local_branches
for branch in $(git branch --merged | egrep -v '(^[*])|(master)|(staging)|(production)|(^\s*$)'); do
  branch_remote=$(git config --get branch.$branch.remote)

  if [ "$branch_remote" == "$remote" ]; then
    branches+=($branch)
  fi

  if [ -z $branch_remote ]; then
    local_branches+=($branch)
  fi
done

if [ ${#branches[*]} -ne 0 ]; then
  echo Cleaning:
  echo ${branches[*]}

  logfile=/tmp/$(basename ${0}).log
  echo -n > $logfile
  for branch in ${branches[*]}; do
    #If we can delete the branch remotely, delete it locally
    git push origin :$branch 2>> $logfile
    git branch -d $branch 2>> $logfile
  done

  cat $logfile
fi

if [ ${#arr[*]} -gt 0 ]; then
  echo "These branches were merged, but don't have a corresponding upstream branch."
  echo ${local_branches[*]}
fi

remotes=$(git remote)
for remote in $remotes; do
  git remote prune $remote
done

echo "Running garbage collection"
git gc 2>/dev/null
*/

func cleanBranches() error {
	return nil
}

func spawnDifftool() error {
	difftool := startCommand("git", "difftool", "-d", "savepoint")
	return difftool.err
}

func runQA() error {
	// the shell implementation looks for environment variables and dispatches to local scripts
	// tempted to do the same thing - the trick is deciding where the demarc is
	log.Print("Unimplemented: still considering how to manage this")
	return nil
}

//cd $(git rev-parse --show-toplevel)
func chDirRepoRoot() (err error) {
	rptl := git("rev-parse", "--show-toplevel")

	if rptl.err != nil {
		err = rptl.err
		return
	}
	toplevelDir := strings.TrimSpace(rptl.stdout)
	os.Chdir(toplevelDir)

	return
}

//git branch savepoint || exit 1
func makeSavepoint() (err error) {
	brsp := git("branch", "savepoint")
	if brsp.err != nil {
		err = brsp.err
	}
	return
}

func recordBranches(branches []string) (err error) {
	file, err := os.Create(branchRecordPath)
	defer file.Close()
	if err != nil {
		return
	}
	enc := json.NewEncoder(file)
	enc.Encode(branches)

	return
}

func mergeBranches(branches []string) (err error) {
	//if [ $# -lt 1 ]; then
	//  echo "Need some origin/branches!"
	//  exit 1
	//fi
	if len(branches) == 0 {
		err = fmt.Errorf("Need some origin/branches!")
		return
	}

	for _, branch := range branches {
		var remote, br string
		remote, br, err = splitBranchname(branch)
		if err != nil {
			return
		}

		pull := git("pull", remote, br)
		err = pull.err
		if err != nil {
			return
		}
	}

	//exec git-savepoint-review
	return
}

var branchSplitRE = regexp.MustCompile(``)

func splitBranchname(name string) (remote, branch string, err error) {
	matches := branchSplitRE.FindStringSubmatch(name)
	if len(matches) < 2 {
		err = fmt.Errorf("Couldn't split %s in remote and branchname", name)
		return
	}

	remote = matches[1]
	branch = matches[2]
	return
}

func git(args ...string) command {
	git := buildCommand("git", args...)
	fmt.Println(git.itself)
	git.run()

	return git
}

type command struct {
	itself         *exec.Cmd
	err            error
	stdout, stderr string
}

func buildCommand(cmdName string, args ...string) command {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var c command
	c.itself = exec.Command(cmdName, args...)
	c.itself.Stdout = &stdout
	c.itself.Stderr = &stderr
	return c
}

func startCommand(cmdName string, args ...string) command {
	c := buildCommand(cmdName, args...)

	c.start()
	return c
}

func runCommand(cmdName string, args ...string) command {
	c := buildCommand(cmdName, args...)

	c.run()
	return c
}

func (c *command) start() error {
	c.err = c.itself.Start()
	return c.err
}

func (c *command) wait() error {
	c.err = c.itself.Wait()
	c.stdout = c.itself.Stdout.(*bytes.Buffer).String()
	c.stderr = c.itself.Stderr.(*bytes.Buffer).String()
	return c.err
}

func (c *command) run() error {
	c.start()
	if c.err != nil {
		return c.err
	}
	c.wait()

	return c.err
}

func (c *command) String() string {
	return fmt.Sprintf("%v %v\nout: %serr: %s", (*c.itself).Args, c.err, c.stdout, c.stderr)
}
