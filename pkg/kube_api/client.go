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
	"errors"
	"fmt"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type K8sAPIClient interface {
	Orchestrate(cfg string, fetchedConfig Result) error
	GetOtelCollectors() (*otelv1alpha1.OpenTelemetryCollector, error)
}

type metadata struct {
	Name string
}
type spec struct {
	Config string
}
type Result struct {
	ApiVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   metadata `json:"metadata"`
	Spec       spec     `json:"spec"`
}

type client struct {
	cf       *rest.Config
	resource dynamic.NamespaceableResourceInterface
}

func NewClient() K8sAPIClient {
	cf, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	dynamicClient, err := dynamic.NewForConfig(cf)
	if err != nil {
		panic(err)
	}
	deploymentRes := schema.GroupVersionResource{
		Group:    "opentelemetry.io",
		Version:  "v1alpha1",
		Resource: "opentelemetrycollectors",
	}
	return &client{
		cf: cf,
		resource: dynamicClient.
			Resource(deploymentRes),
	}
}

func (c *client) Orchestrate(cfg string, fetchedConfig Result) error {
	deployment, err := c.get()
	fmt.Println(err)
	if err != nil {
		if e := c.create(cfg, fetchedConfig); e != nil {
			return e
		}
	} else {
		if e := c.update(deployment, cfg, fetchedConfig); e != nil {
			return e
		}
	}
	return nil
}

func (c *client) GetOtelCollectors() (*otelv1alpha1.OpenTelemetryCollector, error) {
	result, getErr := c.resource.Namespace(apiv1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if getErr != nil {
		panic(fmt.Errorf("failed to get latest version of Deployment: %v", getErr))
	}

	if len(result.Items) == 0 {
		return nil, errors.New("no deployment found")
	}
	return convertUnstructuredToOtelCollector(&result.Items[0])
}

func (c *client) create(cfg string, fetchConfig Result) error {
	otelCol := &otelv1alpha1.OpenTelemetryCollector{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenTelemetryCollector",
			APIVersion: "opentelemetry.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "hello-world",
		},
		Spec: otelv1alpha1.OpenTelemetryCollectorSpec{
			Config:          fetchConfig.Spec.Config,
			Replicas:        int32Ptr(1),
			TargetAllocator: otelv1alpha1.OpenTelemetryTargetAllocator{},
			Mode:            "deployment",
			Resources:       apiv1.ResourceRequirements{},
		},
	}
	deployment, err := convertOtelCollectorToUnstructured(otelCol)
	if err != nil {
		return err
	}
	// Create Deployment
	fmt.Println("Creating deployment...")
	result, err := c.resource.
		Namespace(apiv1.NamespaceDefault).
		Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", result.GetName())
	return nil
}

func int32Ptr(i int32) *int32 { return &i }

func (c *client) update(deployment *unstructured.Unstructured, cfg string, fetchConfig Result) error {
	fmt.Println("Update Resource")
	otelCol, err := convertUnstructuredToOtelCollector(deployment)
	if err != nil {
		return err
	}
	//otelCol.Spec.Config = cfg
	otelCol.Spec.Config = fetchConfig.Spec.Config
	deploymentUpdate, err := convertOtelCollectorToUnstructured(otelCol)
	if err != nil {
		return nil
	}
	_, err = c.resource.
		Namespace(apiv1.NamespaceDefault).
		Update(context.TODO(), deploymentUpdate, metav1.UpdateOptions{})
	fmt.Println("Updated deployment...")

	return err
}

func (c *client) get() (*unstructured.Unstructured, error) {
	fmt.Println("Get Resource")
	options := metav1.GetOptions{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenTelemetryCollector",
			APIVersion: "opentelemetry.io/v1alpha1",
		},
	}
	return c.resource.Namespace(apiv1.NamespaceDefault).Get(context.TODO(), "hello-world", options)
}

func convertOtelCollectorToUnstructured(otelCol *otelv1alpha1.OpenTelemetryCollector) (*unstructured.Unstructured, error) {
	var myMap map[string]interface{}
	data, err := json.Marshal(otelCol)
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

func convertUnstructuredToOtelCollector(deployment *unstructured.Unstructured) (*otelv1alpha1.OpenTelemetryCollector, error) {
	data, err := json.Marshal(deployment.Object)
	if err != nil {
		return nil, err
	}
	otelCol := &otelv1alpha1.OpenTelemetryCollector{}
	if err = json.Unmarshal(data, otelCol); err != nil {
		return nil, err
	}
	return otelCol, nil
}
