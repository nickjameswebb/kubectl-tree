package util

import (
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type UnstructuredTreeNode struct {
	U      *unstructured.Unstructured
	Owners []*UnstructuredTreeNode
}

func (tree *UnstructuredTreeNode) Print(out io.Writer, showAPIVersion bool, indent int) {
	indentation := strings.Repeat(" ", indent)
	if showAPIVersion {
		fmt.Fprintf(out, "%s%s %s %s -n %s\n",
			indentation,
			tree.U.GetAPIVersion(),
			tree.U.GetKind(),
			tree.U.GetName(),
			tree.U.GetNamespace(),
		)
	} else {
		fmt.Fprintf(out, "%s%s %s -n %s\n",
			indentation,
			tree.U.GetKind(),
			tree.U.GetName(),
			tree.U.GetNamespace(),
		)
	}

	for _, owner := range tree.Owners {
		owner.Print(out, showAPIVersion, indent+4)
	}
}
