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
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
)

var (
	treeExample = ``

	// errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")
)

type UserInput struct {
	name           string
	kind           string
	namespace      string
	showAPIVersion bool
}

type TreeOptions struct {
	configFlags *genericclioptions.ConfigFlags
	config      *rest.Config
	client      dynamic.Interface
	restMapper  meta.RESTMapper
	userInput   *UserInput
	args        []string

	genericclioptions.IOStreams
}

func NewTreeOptions(streams genericclioptions.IOStreams) *TreeOptions {
	return &TreeOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:   streams,
		userInput:   &UserInput{},
	}
}

func NewCmdTree(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewTreeOptions(streams)

	cmd := &cobra.Command{
		Use:          "tree",
		Short:        "View a dep tree",
		Example:      treeExample,
		SilenceUsage: false,
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

	cmd.Flags().BoolVar(&o.userInput.showAPIVersion, "show-api-version", false, "Show API Version in output")
	cmd.Flags().StringVar(&o.userInput.name, "name", "", "Name of resource")
	cmd.MarkFlagRequired("name")
	cmd.Flags().StringVarP(&o.userInput.kind, "kind", "", "", "Kind of resource")
	cmd.MarkFlagRequired("kind")
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

	o.userInput.namespace, err = cmd.Flags().GetString("namespace")
	if err != nil {
		return err
	}

	return nil
}

func (o *TreeOptions) Validate() error {
	return nil
}

func (o *TreeOptions) Run() error {
	partialGVR := schema.GroupVersionResource{
		Resource: o.userInput.kind,
	}
	gvr, err := o.restMapper.ResourceFor(partialGVR)
	if err != nil {
		return err
	}

	unstructuredResource, err := o.client.Resource(gvr).Namespace(o.userInput.namespace).Get(context.Background(), o.userInput.name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = o.ktree(unstructuredResource, 0)
	if err != nil {
		return err
	}

	return nil
}

func (o *TreeOptions) ktree(unstructured *unstructured.Unstructured, indent int) error {
	o.printUnstructured(unstructured, indent)

	for _, ownerReference := range unstructured.GetOwnerReferences() {
		partialOwnerReferenceGVR := schema.GroupVersionResource{
			Resource: ownerReference.Kind,
		}
		ownerReferenceGVR, err := o.restMapper.ResourceFor(partialOwnerReferenceGVR)
		if err != nil {
			return err
		}
		unstructuredOwnerReference, err := o.client.Resource(ownerReferenceGVR).Namespace(o.userInput.namespace).Get(context.Background(), ownerReference.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		err = o.ktree(unstructuredOwnerReference, indent+2)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *TreeOptions) printUnstructured(u *unstructured.Unstructured, indent int) {
	indentation := strings.Repeat(" ", indent)
	if o.userInput.showAPIVersion {
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
