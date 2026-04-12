package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// NetworkParameters map to CreateNetworkRequest.
type NetworkParameters struct {
	Name            string  `json:"name"`
	ExternalIP      string  `json:"externalIp"`
	GatewayIP       string  `json:"gatewayIp"`
	InternalNetmask string  `json:"internalNetmask"`
	TargetNode      string  `json:"targetNode,omitempty"`
	AllowedTcpPorts []int32 `json:"allowedTcpPorts,omitempty"`
	AllowedUdpPorts []int32 `json:"allowedUdpPorts,omitempty"`
	VlanID          int32   `json:"vlanId,omitempty"`
	NetworkType     string  `json:"networkType,omitempty"`
	// EnableOutboundNat sets vxlan outbound NAT (proto default is true when omitted).
	EnableOutboundNat bool `json:"enableOutboundNat,omitempty"`
}

// NetworkObservation captures remote fields after create.
type NetworkObservation struct {
	NodeID  string `json:"nodeId,omitempty"`
	Message string `json:"message,omitempty"`
}

// NetworkSpec defines desired state.
type NetworkSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              NetworkParameters `json:"forProvider"`
}

// NetworkStatus is observed state.
type NetworkStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          NetworkObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,kcore}
// Network manages kcore L3/network objects.
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// NetworkList contains Network objects.
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

var (
	NetworkKind             = reflect.TypeOf(Network{}).Name()
	NetworkGroupKind        = schema.GroupKind{Group: Group, Kind: NetworkKind}.String()
	NetworkGroupVersionKind = SchemeGroupVersion.WithKind(NetworkKind)
)

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}
