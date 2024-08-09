package options

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

// Common provides the standard flags and options used in all commands.
type Common struct {
	IOStreams

	// kubernetes repository directory, as specified by the user or current working dir
	RepositoryDir string

	// kubernetes tag, from which to act on
	From string
}

func NewCommon(streams IOStreams) Common {
	return Common{
		IOStreams: streams,
	}
}

func (o *Common) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.RepositoryDir, "repository", o.RepositoryDir, "Kubernetes repository directory, or current directory if none specified")
	flags.StringVar(&o.From, "from", o.From, "Kubernetes version tag from which to start looking for carries (usually the k8s version of the last rebase)")
}

func (o *Common) Complete() error {
	if len(o.RepositoryDir) == 0 {
		var err error
		o.RepositoryDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	if len(o.From) == 0 {
		return fmt.Errorf(`Error: required flag(s) "from" not set`)
	}
	return nil
}
