package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// ContainerParameters mirrors ContainerSpec for workloads.
type ContainerParameters struct {
	Name             string            `json:"name,omitempty"`
	Image            string            `json:"image,omitempty"`
	Network          string            `json:"network,omitempty"`
	Command          []string          `json:"command,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	Ports            []string          `json:"ports,omitempty"`
	StorageBackend   string            `json:"storageBackend,omitempty"`
	StorageSizeBytes int64             `json:"storageSizeBytes,omitempty"`
	MountTarget      string            `json:"mountTarget,omitempty"`
}

// WorkloadParameters are passed to CreateWorkload.
type WorkloadParameters struct {
	// Kind is "vm" or "container".
	Kind             string                   `json:"kind"`
	TargetNode       string                   `json:"targetNode,omitempty"`
	VM               VirtualMachineParameters `json:"vm,omitempty"`
	Container        ContainerParameters      `json:"container,omitempty"`
	ImageURL         string                   `json:"imageUrl,omitempty"`
	ImageSha256      string                   `json:"imageSha256,omitempty"`
	CloudInit        string                   `json:"cloudInitUserData,omitempty"`
	ImagePath        string                   `json:"imagePath,omitempty"`
	ImageFormat      string                   `json:"imageFormat,omitempty"`
	SSHKeyNames      []string                 `json:"sshKeyNames,omitempty"`
	StorageBackend   string                   `json:"storageBackend,omitempty"`
	StorageSizeBytes int64                    `json:"storageSizeBytes,omitempty"`
}

// WorkloadObservation captures observed remote state.
type WorkloadObservation struct {
	WorkloadID     string `json:"workloadId,omitempty"`
	NodeID         string `json:"nodeId,omitempty"`
	VMState        string `json:"vmState,omitempty"`
	ContainerState string `json:"containerState,omitempty"`
	AssignedIP     string `json:"assignedIp,omitempty"`
}

// WorkloadSpec defines desired state.
type WorkloadSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              WorkloadParameters `json:"forProvider"`
}

// WorkloadStatus is observed state.
type WorkloadStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          WorkloadObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,kcore}
// Workload manages a VM or container workload via CreateWorkload.
type Workload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadSpec   `json:"spec"`
	Status WorkloadStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// WorkloadList contains Workload objects.
type WorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workload `json:"items"`
}

var (
	WorkloadKind             = reflect.TypeOf(Workload{}).Name()
	WorkloadGroupKind        = schema.GroupKind{Group: Group, Kind: WorkloadKind}.String()
	WorkloadGroupVersionKind = SchemeGroupVersion.WithKind(WorkloadKind)
)

func init() {
	SchemeBuilder.Register(&Workload{}, &WorkloadList{})
}
