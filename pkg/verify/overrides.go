package verify

import (
	"github.com/openshift/rebase/pkg/carry"
	"github.com/openshift/rebase/pkg/git"
)

// it outputs the commit summaries after applying the overrides
// on top of the original
type overrides struct {
	git     git.Git
	carries []*carry.CommitSummary
}

func (o *overrides) Transform() ([]descriptor, error) {
	return nil, nil
}
