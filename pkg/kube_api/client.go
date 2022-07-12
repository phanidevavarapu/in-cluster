/*
 * Copyright (c) AppDynamics, Inc., and its affiliates 2020
 * All Rights Reserved.
 * THIS IS UNPUBLISHED PROPRIETARY CODE OF APPDYNAMICS, INC.
 *
 * The copyright notice above does not evidence any actual or
 * intended publication of such source code
 */

package kube_api

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"in-cluster/pkg/types"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

type K8sAPIClient interface {
	Orchestrate(cfg string) error
}

type client struct {
	cf            *rest.Config
	dynamicClient dynamic.Interface
}

// NewClient run from a K8s cluster
func NewClient() K8sAPIClient {
	cf, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	dynamicClient, err := dynamic.NewForConfig(cf)
	if err != nil {
		panic(err)
	}
	return &client{
		cf:            cf,
		dynamicClient: dynamicClient,
	}
}

// NewClient2 run from a Local env
func NewClient2() K8sAPIClient {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig1", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	cf, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	dynamicClient, err := dynamic.NewForConfig(cf)
	if err != nil {
		panic(err)
	}
	return &client{
		cf:            cf,
		dynamicClient: dynamicClient,
	}
}

func (c *client) Orchestrate(cfg string) error {
	var appDkube types.AppDKubernetes
	if err := yaml.Unmarshal([]byte(cfg), &appDkube); err != nil {
		return err
	}
	deployed, err := c.get(&appDkube, appDkube.ResourceInfo.OperationInfo.Name)

	if err != nil {
		if e := c.create(&appDkube); e != nil {
			return e
		}
	} else {
		metaData, e := extractMetadata(deployed)
		if e != nil {
			return e
		}
		if e = c.update(&appDkube, metaData); e != nil {
			return e
		}
	}
	return nil
}

func (c *client) create(otelCol *types.AppDKubernetes) error {

	deployment, err := convertOtelCollectorToUnstructured(otelCol)
	if err != nil {
		return err
	}
	// Create Deployment
	fmt.Println("Creating deployment...")
	deploymentRes := schema.GroupVersionResource{
		Group:    otelCol.ResourceInfo.GroupVersionResource.Group,
		Version:  otelCol.ResourceInfo.GroupVersionResource.Version,
		Resource: string(otelCol.ResourceInfo.GroupVersionResource.Resource),
	}
	result, err := c.dynamicClient.Resource(deploymentRes).
		Namespace(apiv1.NamespaceDefault).
		Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	fmt.Printf("Created deployment %q.\n", result.GetName())
	return nil
}

func (c *client) update(otelCol *types.AppDKubernetes, metadata interface{}) error {
	fmt.Println("Update Resource")
	deploymentUpdate, err := convertOtelCollectorToUnstructured(otelCol)
	if err != nil {
		return nil
	}
	deploymentUpdate.Object["metadata"] = mergeMetadata(deploymentUpdate.Object["metadata"], metadata)
	deploymentRes := schema.GroupVersionResource{
		Group:    otelCol.ResourceInfo.GroupVersionResource.Group,
		Version:  otelCol.ResourceInfo.GroupVersionResource.Version,
		Resource: string(otelCol.ResourceInfo.GroupVersionResource.Resource),
	}
	_, err = c.dynamicClient.Resource(deploymentRes).
		Namespace(apiv1.NamespaceDefault).
		Update(context.TODO(), deploymentUpdate, metav1.UpdateOptions{})
	fmt.Println("Updated deployment...")

	return err
}

func (c *client) get(otelCol *types.AppDKubernetes, name string) (*unstructured.Unstructured, error) {
	fmt.Println("Get Resource")
	deploymentRes := schema.GroupVersionResource{
		Group:    otelCol.ResourceInfo.GroupVersionResource.Group,
		Version:  otelCol.ResourceInfo.GroupVersionResource.Version,
		Resource: string(otelCol.ResourceInfo.GroupVersionResource.Resource),
	}
	return c.dynamicClient.Resource(deploymentRes).Namespace(apiv1.NamespaceDefault).Get(context.TODO(), name, metav1.GetOptions{})
}

func convertOtelCollectorToUnstructured(otelCol *types.AppDKubernetes) (*unstructured.Unstructured, error) {
	var myMap map[string]interface{}
	data, err := otelCol.MarshalResourceJSON()
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &myMap); err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{
		Object: myMap,
	}, nil
}

func extractMetadata(deployment *unstructured.Unstructured) (interface{}, error) {
	data, ok := deployment.Object["metadata"]
	if !ok {
		return nil, fmt.Errorf("metadata not found")
	}
	return data, nil
}

func mergeMetadata(current, deployed interface{}) interface{} {
	currentMap := current.(map[string]interface{})
	deployedMap := deployed.(map[string]interface{})
	for k, v := range currentMap {
		if v != nil {
			deployedMap[k] = v
		}
	}
	return deployedMap
}
