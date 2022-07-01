/*
 * Copyright (c) AppDynamics, Inc., and its affiliates 2020
 * All Rights Reserved.
 * THIS IS UNPUBLISHED PROPRIETARY CODE OF APPDYNAMICS, INC.
 *
 * The copyright notice above does not evidence any actual or
 * intended publication of such source code
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	otelv1alpha1 "github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var cfg = `receivers:
  otlp:
    protocols:
      grpc:
      http:
processors:

exporters:
  logging:

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: []
      exporters: [logging]
`

func main() {

	// creates the in-cluster config
	cf, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}

	client, err := dynamic.NewForConfig(cf)
	if err != nil {
		panic(err)
	}

	deploymentRes := schema.GroupVersionResource{
		Group:    "opentelemetry.io",
		Version:  "v1alpha1",
		Resource: "opentelemetrycollectors",
	}

	otelCol := &otelv1alpha1.OpenTelemetryCollector{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenTelemetryCollector",
			APIVersion: "opentelemetry.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "hello-world-3",
		},
		Spec: otelv1alpha1.OpenTelemetryCollectorSpec{
			Config:          cfg,
			Replicas:        int32Ptr(1),
			TargetAllocator: otelv1alpha1.OpenTelemetryTargetAllocator{},
			Mode:            "deployment",
			Resources:       apiv1.ResourceRequirements{},
		},
	}
	var myMap map[string]interface{}
	data, _ := json.Marshal(otelCol)
	json.Unmarshal(data, &myMap)
	deployment := &unstructured.Unstructured{
		Object: myMap,
	}
	// Create Deployment
	fmt.Println("Creating deployment...")
	result, err := client.
		Resource(deploymentRes).
		Namespace(apiv1.NamespaceDefault).
		Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created deployment %q.\n", result.GetName())
}

func int32Ptr(i int32) *int32 { return &i }
