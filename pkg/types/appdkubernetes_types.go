/*
 * Copyright (c) AppDynamics, Inc., and its affiliates 2020
 * All Rights Reserved.
 * THIS IS UNPUBLISHED PROPRIETARY CODE OF APPDYNAMICS, INC.
 *
 * The copyright notice above does not evidence any actual or
 * intended publication of such source code
 */

package types

import (
	"encoding/json"
	"errors"
	"github.com/open-telemetry/opentelemetry-operator/apis/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
)

// KubernetesAPI is placeholder for the Kubernetes kind api
type KubernetesAPI struct {
	Pod                   *apiv1.Pod                   `json:"pod,omitempty"`
	Node                  *apiv1.Node                  `json:"node,omitempty"`
	Service               *apiv1.Service               `json:"service,omitempty"`
	Namespace             *apiv1.Namespace             `json:"namespace,omitempty"`
	LimitRange            *apiv1.LimitRange            `json:"limitRange,omitempty"`
	ResourceQuota         *apiv1.ResourceQuota         `json:"resourceQuota,omitempty"`
	PersistentVolume      *apiv1.PersistentVolume      `json:"persistentVolume,omitempty"`
	PersistentVolumeClaim *apiv1.PersistentVolumeClaim `json:"persistentVolumeClaim,omitempty"`
	ReplicationController *apiv1.ReplicationController `json:"replicationController,omitempty"`
}

// KubernetesApps is placeholder for the Kubernetes Resource apps
type KubernetesApps struct {
	DaemonSet   *appsv1.DaemonSet   `json:"daemonSet,omitempty"`
	ReplicaSet  *appsv1.ReplicaSet  `json:"replicaSet,omitempty"`
	Deployment  *appsv1.Deployment  `json:"deployment,omitempty"`
	StatefulSet *appsv1.StatefulSet `json:"statefulSet,omitempty"`
}

// KubernetesCRD is placeholder for the Kubernetes Resource CRDS
type KubernetesCRD struct {
	OpenTelemetryCollector *v1alpha1.OpenTelemetryCollector `json:"openTelemetryCollector,omitempty"`
}

type Kind string

const (
	// Kinds of type API

	Pods                   Kind = "pods"
	Nodes                  Kind = "nodes"
	Services               Kind = "services"
	Namespaces             Kind = "namespaces"
	LimitRanges            Kind = "limitranges"
	ResourceQuotas         Kind = "resourcequotas"
	PersistentVolumes      Kind = "persistentvolumes"
	PersistentVolumeClaims Kind = "persistentvolumeclaims"
	ReplicationControllers Kind = "replicationcontrollers"

	DaemonSets   Kind = "daemonsets"
	ReplicaSets  Kind = "replicasets"
	Deployments  Kind = "deployments"
	StatefulSets Kind = "statefulsets"

	OpenTelemetryCollectors Kind = "opentelemetrycollectors"
)

// GroupVersionResource unambiguously identifies a resource.  It doesn't anonymously include GroupVersion
// to avoid automatic coercion.  It doesn't use a GroupVersion to avoid custom marshalling
type GroupVersionResource struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource Kind   `json:"resource"`
}

type Operation int

const (
	Create Operation = iota + 1
	Update
	Delete
)

type OperationInfo struct {
	Name      string    `json:"name"`
	Operation Operation `json:"operation"`
}

type ResourceInfo struct {
	OperationInfo        OperationInfo        `json:"operationInfo"`
	GroupVersionResource GroupVersionResource `json:"groupVersionResource"`
}

// AppDKubernetes placeholder for different types of resources such as api, apps, CRDS
type AppDKubernetes struct {
	ResourceInfo    `json:",inline"`
	*KubernetesAPI  `json:",inline,omitempty"`
	*KubernetesApps `json:",inline,omitempty"`
	*KubernetesCRD  `json:",inline,omitempty"`
}

// MarshalResourceJSON to convert resource object to Json
func (a *AppDKubernetes) MarshalResourceJSON() ([]byte, error) {
	switch a.ResourceInfo.GroupVersionResource.Resource {
	case Pods:
		return json.Marshal(a.KubernetesAPI.Pod)
	case Nodes:
		return json.Marshal(a.KubernetesAPI.Node)
	case Services:
		return json.Marshal(a.KubernetesAPI.Service)
	case Namespaces:
		return json.Marshal(a.KubernetesAPI.Namespace)
	case LimitRanges:
		return json.Marshal(a.KubernetesAPI.LimitRange)
	case ResourceQuotas:
		return json.Marshal(a.KubernetesAPI.ResourceQuota)
	case PersistentVolumes:
		return json.Marshal(a.KubernetesAPI.PersistentVolume)
	case PersistentVolumeClaims:
		return json.Marshal(a.KubernetesAPI.PersistentVolumeClaim)
	case ReplicationControllers:
		return json.Marshal(a.KubernetesAPI.ReplicationController)

	case DaemonSets:
		return json.Marshal(a.KubernetesApps.DaemonSet)
	case ReplicaSets:
		return json.Marshal(a.KubernetesApps.ReplicaSet)
	case StatefulSets:
		return json.Marshal(a.KubernetesApps.StatefulSet)
	case Deployments:
		return json.Marshal(a.KubernetesApps.Deployment)

	case OpenTelemetryCollectors:
		return json.Marshal(a.KubernetesCRD.OpenTelemetryCollector)

	default:
		return nil, errors.New("un-know resource type")
	}
}
