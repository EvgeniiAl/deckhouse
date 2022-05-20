/*
Copyright 2021 Flant JSC

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Descheduler is a group of nodes in Kubernetes.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:JSONPath=status.ready,name=Ready,type=boolean
type Descheduler struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of a descheduler instance.
	Spec DeschedulerSpec `json:"spec"`

	// Most recently observed status of a descheduler instance.
	Status DeschedulerStatus `json:"status,omitempty"`
}

type DeschedulerSpec struct {
	DeploymentTemplate DeschedulerDeploymentTemplate `json:"deploymentTemplate,omitempty"`

	DeschedulerPolicy DeschedulerPolicy `json:"deschedulerPolicy,omitempty"`
}

type DeschedulerDeploymentTemplate struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	ResourcesRequests *ResourcesRequests `json:"resourcesRequests,omitempty"`
}

type ResourcesRequests struct {
	// +kubebuilder:default=VPA
	// +kubebuilder:validation:Enum=Manual;VPA;Off
	Mode ResourcesRequestsMode `json:"mode,omitempty"`
}

type ResourcesRequestsMode string

const (
	ResourcesRequestsModeManual = "Manual"
	ResourcesRequestsModeVPA    = "VPA"
	ResourcesRequestsModeOff    = "Off"
)

type DeschedulerPolicy struct {
	CommonParameters CommonParameters `json:"parameters,omitempty"`

	Strategies DeschedulerStrategies `json:"strategies,omitempty"`
}

type CommonParameters struct {
	// NodeSelector for a set of nodes to operate over
	NodeSelector *string `json:"nodeSelector,omitempty"`

	// EvictFailedBarePods allows pods without ownerReferences and in failed phase to be evicted.
	EvictFailedBarePods *bool `json:"evictFailedBarePods,omitempty"`

	// EvictLocalStoragePods allows pods using local storage to be evicted.
	EvictLocalStoragePods *bool `json:"evictLocalStoragePods,omitempty"`

	// EvictSystemCriticalPods allows eviction of pods of any priority (including Kubernetes system pods)
	// Note: Setting evictSystemCriticalPods to true disables priority filtering entirely.
	EvictSystemCriticalPods *bool `json:"evictSystemCriticalPods,omitempty"`

	// IgnorePVCPods prevents pods with PVCs from being evicted.
	IgnorePVCPods *bool `json:"ignorePvcPods,omitempty"`

	// MaxNoOfPodsToEvictPerNode restricts maximum of pods to be evicted per node.
	MaxNoOfPodsToEvictPerNode *int `json:"maxNoOfPodsToEvictPerNode,omitempty"`

	// MaxNoOfPodsToEvictPerNamespace restricts maximum of pods to be evicted per namespace.
	MaxNoOfPodsToEvictPerNamespace *int `json:"maxNoOfPodsToEvictPerNamespace,omitempty"`
}

type DeschedulerStrategies struct {
	RemoveDuplicates                            *RemoveDuplicates                            `json:"removeDuplicates,omitempty"`
	LowNodeUtilization                          *LowNodeUtilization                          `json:"lowNodeUtilization,omitempty"`
	HighNodeUtilization                         *HighNodeUtilization                         `json:"highNodeUtilization,omitempty"`
	RemovePodsViolatingInterPodAntiAffinity     *RemovePodsViolatingInterPodAntiAffinity     `json:"removePodsViolatingInterPodAntiAffinity,omitempty"`
	RemovePodsViolatingNodeAffinity             *RemovePodsViolatingNodeAffinity             `json:"removePodsViolatingNodeAffinity,omitempty"`
	RemovePodsViolatingNodeTaints               *RemovePodsViolatingNodeTaints               `json:"removePodsViolatingNodeTaints,omitempty"`
	RemovePodsViolatingTopologySpreadConstraint *RemovePodsViolatingTopologySpreadConstraint `json:"removePodsViolatingTopologySpreadConstraint,omitempty"`
	RemovePodsHavingTooManyRestarts             *RemovePodsHavingTooManyRestarts             `json:"removePodsHavingTooManyRestarts,omitempty"`
	PodLifeTime                                 *PodLifeTime                                 `json:"podLifeTime,omitempty"`
	RemoveFailedPods                            *RemoveFailedPods                            `json:"removeFailedPods,omitempty"`
}

type RemoveDuplicates struct {
	ThresholdPrioritiesFiltering `json:",inline"`
	NamespacesFiltering          `json:",inline"`
	NodeFitFiltering             `json:",inline"`

	RemoveDuplicates *RemoveDuplicatesParameters `json:"removeDuplicates,omitempty"`
}

type LowNodeUtilization struct {
	NodeFitFiltering `json:",inline"`

	NodeResourceUtilizationThresholds *NodeResourceUtilizationThresholdsFiltering `json:"nodeResourceUtilizationThresholds,omitempty"`
}

type HighNodeUtilization struct {
	NodeFitFiltering `json:",inline"`

	NodeResourceUtilizationThresholds *NodeResourceUtilizationThresholdsFiltering `json:"nodeResourceUtilizationThresholds,omitempty"`
}

type RemovePodsViolatingInterPodAntiAffinity struct {
	ThresholdPrioritiesFiltering `json:",inline"`
	NamespacesFiltering          `json:",inline"`
	LabelSelectorFiltering       `json:",inline"`
	NodeFitFiltering             `json:",inline"`
}

type RemovePodsViolatingNodeAffinity struct {
	ThresholdPrioritiesFiltering `json:",inline"`
	NamespacesFiltering          `json:",inline"`
	LabelSelectorFiltering       `json:",inline"`
	NodeFitFiltering             `json:",inline"`

	NodeAffinityType []string `json:"nodeAffinityType,omitempty"`
}

type RemovePodsViolatingNodeTaints struct {
	ThresholdPrioritiesFiltering `json:",inline"`
	NamespacesFiltering          `json:",inline"`
	LabelSelectorFiltering       `json:",inline"`
	NodeFitFiltering             `json:",inline"`

	ExcludedTaints []string `json:"excludedTaints,omitempty"`
}

type RemovePodsViolatingTopologySpreadConstraint struct {
	ThresholdPrioritiesFiltering `json:",inline"`
	NamespacesFiltering          `json:",inline"`
	LabelSelectorFiltering       `json:",inline"`
	NodeFitFiltering             `json:",inline"`

	IncludeSoftConstraints bool `json:"includeSoftConstraints"`
}

type RemovePodsHavingTooManyRestarts struct {
	ThresholdPrioritiesFiltering `json:",inline"`
	NamespacesFiltering          `json:",inline"`
	LabelSelectorFiltering       `json:",inline"`
	NodeFitFiltering             `json:",inline"`

	PodsHavingTooManyRestarts *PodsHavingTooManyRestartsParameters `json:"podsHavingTooManyRestarts,omitempty"`
}

type PodLifeTime struct {
	ThresholdPrioritiesFiltering `json:",inline"`
	NamespacesFiltering          `json:",inline"`
	LabelSelectorFiltering       `json:",inline"`

	PodLifeTime *PodLifeTimeParameters `json:"podLifeTime,omitempty"`
}

type RemoveFailedPods struct {
	ThresholdPrioritiesFiltering `json:",inline"`
	NamespacesFiltering          `json:",inline"`
	LabelSelectorFiltering       `json:",inline"`
	NodeFitFiltering             `json:",inline"`

	RemoveFailedPods *RemoveFailedPodsParameters `json:"removeFailedPods,omitempty"`
}

type RemoveDuplicatesParameters struct {
	ExcludeOwnerKinds []string `json:"excludeOwnerKinds,omitempty"`
}

type PodsHavingTooManyRestartsParameters struct {
	PodRestartThreshold     int32 `json:"podRestartThreshold,omitempty"`
	IncludingInitContainers bool  `json:"includingInitContainers,omitempty"`
}

type PodLifeTimeParameters struct {
	MaxPodLifeTimeSeconds *uint    `json:"maxPodLifeTimeSeconds,omitempty"`
	PodStatusPhases       []string `json:"podStatusPhases,omitempty"`
}

type RemoveFailedPodsParameters struct {
	ExcludeOwnerKinds       []string `json:"excludeOwnerKinds,omitempty"`
	MinPodLifetimeSeconds   *uint    `json:"minPodLifetimeSeconds,omitempty"`
	Reasons                 []string `json:"reasons,omitempty"`
	IncludingInitContainers bool     `json:"includingInitContainers,omitempty"`
}

// NamespacesFiltering carries a list of included/excluded namespaces
// for which a given strategy is applicable.
type NamespacesFiltering struct {
	Namespaces `json:"namespaces,omitempty"`
}

type Namespaces struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

type ThresholdPrioritiesFiltering struct {
	ThresholdPriority          *int32 `json:"thresholdPriority"`
	ThresholdPriorityClassName string `json:"thresholdPriorityClassName"`
}

type LabelSelectorFiltering struct {
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

type NodeFitFiltering struct {
	NodeFit bool `json:"nodeFit,omitempty"`
}

type NodeResourceUtilizationThresholdsFiltering struct {
	UseDeviationThresholds bool               `json:"useDeviationThresholds,omitempty"`
	Thresholds             ResourceThresholds `json:"thresholds,omitempty"`
	TargetThresholds       ResourceThresholds `json:"targetThresholds,omitempty"`
	NumberOfNodes          int                `json:"numberOfNodes,omitempty"`
}

type Percentage float64
type ResourceThresholds map[corev1.ResourceName]Percentage

type DeschedulerStatus struct {
	Ready bool `json:"ready"`
}