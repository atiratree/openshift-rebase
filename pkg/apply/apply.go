package apply

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/openshift/rebase/pkg/carry"
	"github.com/openshift/rebase/pkg/git"
	"github.com/openshift/rebase/pkg/utils"
	"k8s.io/klog/v2"
)

type Apply struct {
	log           *carry.Log
	from          string
	repositoryDir string
}

var (
	actionRE = regexp.MustCompile(`UPSTREAM: (?P<action>[<>\w]+):`)
)

func NewApply(from, repositoryDir string) *Apply {
	return &Apply{
		log:           carry.NewLog(from, repositoryDir),
		from:          from,
		repositoryDir: repositoryDir,
	}
}

func (c *Apply) Run() error {
	// this applies the steps from https://github.com/openshift/kubernetes/blob/master/REBASE.openshift.md
	repository, err := git.OpenGit(c.repositoryDir)
	if err != nil {
		return err
	}
	// TODO: add fetching remotes
	commits, err := c.log.GetCommits(repository)
	if err != nil {
		return fmt.Errorf("Error reading carries: %w", err)
	}
	branchName := fmt.Sprintf("rebase-%s", time.Now().Format(time.DateOnly))
	if err := repository.CreateBranch(branchName, "refs/remotes/upstream/master"); err != nil {
		return fmt.Errorf("Error creating rebase branch: %w", err)
	}
	if err := repository.Merge("openshift/master"); err != nil {
		return fmt.Errorf("Error creating rebase branch: %w", err)
	}
	for _, c := range commits {
		klog.V(2).Infof("Processing %s: %q", c.Hash.String(), utils.FormatMessage(c.Message))
		action := actionFromMessage(utils.FormatMessage(c.Message))
		if _, err := strconv.Atoi(action); err == nil {
			// TODO: upstream pick, for now handle as carry
			action = "<carry>"
		}
		switch action {
		case "<carry>":
			if err := carryFlow(repository, c); err != nil {
				// TODO: abort only after 2-3 errors, maybe?
				return err
			}
		case "<drop>":
			klog.Infof("Dropping commit %s.", c.Hash.String())
		default:
			klog.Infof("Unkown action on commit %s: %s", c.Hash.String(), action)
		}
	}
	return nil
}

// carryFlow implements the carry action
func carryFlow(repository git.Git, commit *object.Commit) error {
	klog.V(2).Infof("Initiating carry flow for %s...", commit.Hash.String())
	if err := repository.CherryPick(commit.Hash.String()); err == nil {
		return nil
	}
	klog.Infof("Encountered problems picking %s:", commit.Hash.String())
	if err := repository.Status(); err != nil {
		return err
	}
	if err := repository.AbortCherryPick(); err != nil {
		return err
	}
	klog.V(2).Infof("Looking for a fixed carry")
	patch, err := findFixedCarry(commit.Hash.String())
	if err != nil {
		klog.Errorf("Carry https://github.com/openshift/kubernetes/commit/%s requires manual intervention!", commit.Hash.String())
		return err
	}
	klog.Infof("Found %s, applying...", patch)
	if err := repository.Apply(patch); err != nil {
		return err
	}
	return nil
}

// actionFromMessage parses the upstream action from commit message, returning
// which action to take on a commit
func actionFromMessage(message string) string {
	matches := actionRE.FindStringSubmatch(message)
	lastIndex := actionRE.SubexpIndex("action")
	if lastIndex < 0 {
		return ""
	}
	return matches[lastIndex]
}

// findFixedCarry looks for fixed carry patches. Returns path to a file containing
// the carry or error.
func findFixedCarry(carrySha string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	carryPath := path.Join(cwd, "carries", carrySha)
	if _, err := os.Stat(carryPath); err != nil {
		return "", err
	}
	return carryPath, nil
}
