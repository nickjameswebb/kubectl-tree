package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/nickjameswebb/kubectl-tree/pkg/cmd"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-tree", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewCmdTree(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "kubectl-tree failed: %v\n", err)
		os.Exit(1)
	}
}
