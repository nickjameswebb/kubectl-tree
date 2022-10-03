/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

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

// TODO: write out examples, usage
var (
	treeExample = ``
)

type TreeOptions struct {
	configFlags *genericclioptions.ConfigFlags
	config      *rest.Config
	client      dynamic.Interface
	restMapper  meta.RESTMapper
	args        []string
	builder     *resource.Builder

	// TODO: implement AllNamespaces
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
		Use:          "tree",
		Short:        "View a dep tree",
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
		NamespaceParam(o.namespace).
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

		err = o.ktree(u, 0)
		if err != nil {
			return err
		}

		return nil
	})
}

// TODO: doesn't work with cluster level owner
// TODO: can we use concurrency, builder, etc. for this
func (o *TreeOptions) ktree(u *unstructured.Unstructured, indent int) error {
	o.printUnstructured(u, indent)

	for _, ownerReference := range u.GetOwnerReferences() {
		ownerReferenceGVR, err := o.getOwnerReferenceGVR(ownerReference)
		if err != nil {
			return err
		}

		isNamespaced, err := o.groupKindIsNamespaced(schema.GroupKind{
			Group: ownerReferenceGVR.Group,
			Kind:  ownerReference.Kind,
		})
		if err != nil {
			return err
		}

		namespace := ""
		if isNamespaced {
			namespace = o.namespace
		}

		unstructuredOwnerReference, err := o.client.
			Resource(ownerReferenceGVR).
			Namespace(namespace).
			Get(context.Background(), ownerReference.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		err = o.ktree(unstructuredOwnerReference, indent+4)
		if err != nil {
			return err
		}
	}

	return nil
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

func (o *TreeOptions) printUnstructured(u *unstructured.Unstructured, indent int) {
	indentation := strings.Repeat(" ", indent)
	if o.showAPIVersion {
		fmt.Fprintf(o.Out, "%s%s %s %s -n %s\n",
			indentation,
			u.GetAPIVersion(),
			u.GetKind(),
			u.GetName(),
			u.GetNamespace(),
		)
	} else {
		fmt.Fprintf(o.Out, "%s%s %s -n %s\n",
			indentation,
			u.GetKind(),
			u.GetName(),
			u.GetNamespace(),
		)
	}

}
