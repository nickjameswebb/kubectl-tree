package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/nickjameswebb/kubectl-tree/pkg/util"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
)

// TODO: do I need to implement all the inherited flags?
var (
	treeUse = "tree"

	treeShort = "View the ownership tree of a resource"

	treeExample = `
	# view an ownership tree of all pods in kube-system namespace
	kubectl tree pods -n kube-system

	# view an ownership tree of all pods in all namespaces
	kubectl tree pods -A

	# view an ownership tree of a specific pod in kube-system namespace
	kubectl tree pods/foo -n kube-system

	# optionally display API version in ownership tree
	kubectl tree pods/foo -n kube-system --show-api-version=true
`
)

type TreeOptions struct {
	configFlags *genericclioptions.ConfigFlags
	config      *rest.Config
	client      dynamic.Interface
	restMapper  meta.RESTMapper
	args        []string
	builder     *resource.Builder

	allNamespaces  bool
	namespace      string
	showAPIVersion bool

	genericclioptions.IOStreams
}

func NewTreeOptions(streams genericclioptions.IOStreams) *TreeOptions {
	return &TreeOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:   streams,
	}
}

func NewCmdTree(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewTreeOptions(streams)

	cmd := &cobra.Command{
		Use:          treeUse,
		Short:        treeShort,
		Example:      treeExample,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&o.allNamespaces, "all-namespaces", "A", o.allNamespaces, "If present, view a tree for object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&o.showAPIVersion, "show-api-version", false, "Show API Version in output")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

func (o *TreeOptions) Complete(cmd *cobra.Command, args []string) error {
	o.args = args

	var err error
	o.restMapper, err = o.configFlags.ToRESTMapper()
	if err != nil {
		return err
	}

	o.config, err = o.configFlags.ToRESTConfig()
	if err != nil {
		return err
	}

	o.client, err = dynamic.NewForConfig(o.config)
	if err != nil {
		return err
	}

	if o.configFlags.Namespace != nil {
		o.namespace = *o.configFlags.Namespace
	}

	o.builder = resource.NewBuilder(o.configFlags)

	return nil
}

func (o *TreeOptions) Validate() error {
	return nil
}

func (o *TreeOptions) Run() error {
	r := o.builder.
		Unstructured().
		NamespaceParam(o.namespace).DefaultNamespace().
		AllNamespaces(o.allNamespaces).
		ResourceTypeOrNameArgs(true, o.args...).
		Flatten().
		Latest().
		Do()
	if err := r.Err(); err != nil {
		return err
	}

	return r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		u := &unstructured.Unstructured{}
		if err := scheme.Scheme.Convert(info.Object, u, nil); err != nil {
			return err
		}

		tree, err := o.buildUnstructuredTree(u)
		if err != nil {
			return err
		}

		tree.Print(o.Out, o.showAPIVersion, 0)

		return nil
	})
}

// TODO: can we use concurrency, builder, etc. for this
func (o *TreeOptions) buildUnstructuredTree(u *unstructured.Unstructured) (*util.UnstructuredTreeNode, error) {
	tree := &util.UnstructuredTreeNode{
		U:      u,
		Owners: []*util.UnstructuredTreeNode{},
	}

	for _, ownerReference := range u.GetOwnerReferences() {
		ownerReferenceGVR, err := o.getOwnerReferenceGVR(ownerReference)
		if err != nil {
			return nil, err
		}

		isNamespaced, err := o.groupKindIsNamespaced(schema.GroupKind{
			Group: ownerReferenceGVR.Group,
			Kind:  ownerReference.Kind,
		})
		if err != nil {
			return nil, err
		}

		namespace := ""
		if isNamespaced {
			namespace = u.GetNamespace()
		}

		unstructuredOwnerReference, err := o.client.
			Resource(ownerReferenceGVR).
			Namespace(namespace).
			Get(context.Background(), ownerReference.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		subtree, err := o.buildUnstructuredTree(unstructuredOwnerReference)
		if err != nil {
			return nil, err
		}

		tree.Owners = append(tree.Owners, subtree)
	}

	return tree, nil
}

func (o *TreeOptions) groupKindIsNamespaced(groupKind schema.GroupKind) (bool, error) {
	restMapping, err := o.restMapper.RESTMapping(groupKind)
	if err != nil {
		return false, err
	}

	return restMapping.Scope.Name() == "namespace", nil
}

func (o *TreeOptions) getOwnerReferenceGVR(ownerReference metav1.OwnerReference) (schema.GroupVersionResource, error) {
	partialOwnerReferenceGVR := schema.GroupVersionResource{
		Resource: ownerReference.Kind,
	}
	return o.restMapper.ResourceFor(partialOwnerReferenceGVR)
}
