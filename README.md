```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func newDynamicClient() (dynamic.Interface, error) {
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	kubeConfigPath := filepath.Join(userHomeDir, ".kube", "config")
	log.Printf("Using kubeconfig: %s\n", kubeConfigPath)

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}

	client, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func get(client dynamic.Interface, name string, kind string, namespace string, apiVersion string, group string) (*unstructured.Unstructured, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  apiVersion,
		Resource: kind,
	}

	log.Printf("get name=%s,kind=%s,namespace=%s,apiVersion=%s,group=%s\n", name, kind, namespace, apiVersion, group)

	return client.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func ktree(client dynamic.Interface, unstructured *unstructured.Unstructured, indent int) error {
	fmt.Printf("%sname=%s,kind=%s,apiVersion=%s\n", strings.Repeat(" ", indent), unstructured.GetName(), unstructured.GetKind(), unstructured.GetAPIVersion())

	for _, ownerReference := range unstructured.GetOwnerReferences() {
		ownerRefApiVersion, ownerRefGroup, err := splitApiVersion(ownerReference.APIVersion)
		if err != nil {
			return err
		}
		ownerRefUnstructured, err := get(client, ownerReference.Name, ownerReference.Kind, unstructured.GetNamespace(), ownerRefApiVersion, ownerRefGroup)
		if err != nil {
			return err
		}
		err = ktree(client, ownerRefUnstructured, indent+2)
		if err != nil {
			return err
		}
	}

	return nil
}

func splitApiVersion(fullApiVersion string) (string, string, error) {
	slices := strings.Split(fullApiVersion, "/")
	if len(slices) == 1 {
		return slices[0], "", nil
	} else if len(slices) == 2 {
		return slices[1], slices[0], nil
	} else {
		return "", "", errors.New("invalid api version")
	}
}

func main() {
	args := os.Args[1:]

	if len(args) != 4 {
		log.Fatalf("usage\n")
	}

	name := args[0]
	kind := args[1]
	namespace := args[2]
	apiVersion := args[3]

	client, err := newDynamicClient()
	if err != nil {
		log.Fatalf("error building kubeconfig: %v\n", err)
	}

	apiVersion, group, err := splitApiVersion(apiVersion)
	if err != nil {
		log.Fatalf("error apiVersion: %v\n", err)
	}

	unstructured, err := get(client, name, kind, namespace, apiVersion, group)
	if err != nil {
		log.Fatalf("error getting resource: %v\n", err)
	}

	err = ktree(client, unstructured, 0)
	if err != nil {
		log.Fatalf("error building ktree: %v\n", err)
	}
}
```