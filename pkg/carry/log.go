package carry

import (
	"fmt"
	"slices"
	"strings"
	"time"

	gitv5object "github.com/go-git/go-git/v5/plumbing/object"
	"github.com/openshift/rebase/pkg/git"
	"github.com/openshift/rebase/pkg/utils"
	"k8s.io/klog/v2"
)

const (
	rebaseMarker   = `Merge remote-tracking branch 'openshift/master' into`
	mergeMarker    = `Merge pull request #`
	upstreamPrefix = "UPSTREAM: "
)

type Log struct {
	from          string
	repositoryDir string
}

func NewLog(from, repositoryDir string) *Log {
	return &Log{
		from:          from,
		repositoryDir: repositoryDir,
	}
}

func (c *Log) Run() error {
	repository, err := git.OpenGit(c.repositoryDir)
	if err != nil {
		return err
	}
	commits, err := c.GetCommits(repository)
	if err != nil {
		return fmt.Errorf("Error reading carries: %w", err)
	}
	for _, c := range commits {
		fmt.Printf("%s\t%s\t%-25s\t%s\t%s\n", c.Committer.When.Format(time.DateTime),
			c.Author.When.Format(time.DateTime),
			c.Author.Name, c.Hash.String(), utils.FormatMessage(c.Message))
	}
	return nil
}

func (c *Log) GetCommits(repository git.Git) ([]*gitv5object.Commit, error) {
	if err := repository.Checkout("openshift/master"); err != nil {
		return nil, err
	}
	commits, err := repository.LogFromTag(c.from)
	if err != nil {
		return nil, err
	}
	// LogFromTag uses gitv5.LogOrderCommitterTime order, but we need the oldest first
	slices.Reverse(commits)

	foundRebaseMarker := false
	var carryCommits []*gitv5object.Commit
	for _, c := range commits {
		klog.V(5).Infof("Processing %s", c)
		if !foundRebaseMarker {
			if strings.Contains(c.Message, rebaseMarker) {
				klog.V(2).Infof("Found rebase marker at %s", c)
				foundRebaseMarker = true
			}
			continue
		}
		if strings.Contains(c.Message, mergeMarker) {
			// TODO: check if the commit being brought by merge commit is already included, this currently produces duplicates
			// for some of the commits, but is required since some commits might be created before the rebase landed,
			// but merged afterwards, so the --since in log will skip them, a good example from 4.11/1.24.is:
			// 2022-06-09 00:31:24 -0400 -0400 OpenShift Merge Robot Merge pull request #1229 from rphillips/backports/109103 cb7147853d28e94e1e32674d535e53aec4d9946f
			// 2022-03-29 23:53:02 +0800 +0800 DingShujie UPSTREAM: 109103: cpu manager policy set to none, no one remove container id from container map, lea ed4d3f61aaccbc2fbe383c4d6b9614e8d2ad3e16
			for _, hash := range c.ParentHashes {
				ci, err := repository.Commit(hash)
				if err != nil {
					klog.Errorf("error reading commit %s: %v", hash, err)
					continue
				}
				if strings.Contains(ci.Message, mergeMarker) || !strings.Contains(ci.Message, upstreamPrefix) {
					continue
				}
				carryCommits = append(carryCommits, ci)
			}
			continue
		}
		if !strings.Contains(c.Message, upstreamPrefix) {
			continue
		}
		carryCommits = append(carryCommits, c)
	}

	return deduplicateCommits(carryCommits), nil
}

// deduplicateCommits is responsible for dropping duplicate commits from the result list,
// but without modifying the original order of commits
func deduplicateCommits(commits []*gitv5object.Commit) []*gitv5object.Commit {
	uniqueSha := make(map[string]bool)
	var filteredCommits []*gitv5object.Commit
	for _, c := range commits {
		if _, exists := uniqueSha[c.Hash.String()]; exists {
			continue
		}
		uniqueSha[c.Hash.String()] = true
		filteredCommits = append(filteredCommits, c)
	}
	return filteredCommits
}
