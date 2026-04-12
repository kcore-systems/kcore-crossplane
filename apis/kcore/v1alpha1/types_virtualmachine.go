package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// Disk mirrors kcore Disk.
type Disk struct {
	Name          string `json:"name,omitempty"`
	BackendHandle string `json:"backendHandle,omitempty"`
	Bus           string `json:"bus,omitempty"`
	Device        string `json:"device,omitempty"`
}

// Nic mirrors kcore Nic.
type Nic struct {
	Network    string `json:"network,omitempty"`
	Model      string `json:"model,omitempty"`
	MacAddress string `json:"macAddress,omitempty"`
}

// VirtualMachineParameters are fields passed to CreateVm.
type VirtualMachineParameters struct {
	TargetNode string `json:"targetNode,omitempty"`

	Name        string   `json:"name"`
	CPUs        int32    `json:"cpus"`
	MemoryBytes int64    `json:"memoryBytes"`
	Disks       []Disk   `json:"disks,omitempty"`
	Nics        []Nic    `json:"nics,omitempty"`
	ImageURL    string   `json:"imageUrl,omitempty"`
	ImageSha256 string   `json:"imageSha256,omitempty"`
	CloudInit   string   `json:"cloudInitUserData,omitempty"`
	ImagePath   string   `json:"imagePath,omitempty"`
	ImageFormat string   `json:"imageFormat,omitempty"`
	SSHKeyNames []string `json:"sshKeyNames,omitempty"`
	// StorageBackend is one of: unspecified, filesystem, lvm, zfs (matches proto enum names).
	StorageBackend   string `json:"storageBackend,omitempty"`
	StorageSizeBytes int64  `json:"storageSizeBytes"`
}

// VirtualMachineObservation captures observed state from GetVm.
type VirtualMachineObservation struct {
	VMID       string `json:"vmId,omitempty"`
	NodeID     string `json:"nodeId,omitempty"`
	State      string `json:"state,omitempty"`
	AssignedIP string `json:"assignedIp,omitempty"`
}

// VirtualMachineSpec defines the desired state of a VirtualMachine.
type VirtualMachineSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              VirtualMachineParameters `json:"forProvider"`
}

// VirtualMachineStatus represents observed state.
type VirtualMachineStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          VirtualMachineObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,kcore}
// VirtualMachine manages a kcore VM via the controller API.
type VirtualMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineSpec   `json:"spec"`
	Status VirtualMachineStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// VirtualMachineList contains VirtualMachine resources.
type VirtualMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachine `json:"items"`
}

var (
	VirtualMachineKind             = reflect.TypeOf(VirtualMachine{}).Name()
	VirtualMachineGroupKind        = schema.GroupKind{Group: Group, Kind: VirtualMachineKind}.String()
	VirtualMachineGroupVersionKind = SchemeGroupVersion.WithKind(VirtualMachineKind)
)

func init() {
	SchemeBuilder.Register(&VirtualMachine{}, &VirtualMachineList{})
}
