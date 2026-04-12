package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// SSHKeyParameters map to CreateSshKeyRequest.
type SSHKeyParameters struct {
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"`
}

// SSHKeyObservation captures observed remote metadata.
type SSHKeyObservation struct {
	Name      string `json:"name,omitempty"`
	PublicKey string `json:"publicKey,omitempty"`
}

// SSHKeySpec defines desired state.
type SSHKeySpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              SSHKeyParameters `json:"forProvider"`
}

// SSHKeyStatus is observed state.
type SSHKeyStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SSHKeyObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,kcore}
// SSHKey registers an SSH public key with the kcore controller.
type SSHKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SSHKeySpec   `json:"spec"`
	Status SSHKeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// SSHKeyList contains SSHKey objects.
type SSHKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSHKey `json:"items"`
}

var (
	SSHKeyKind             = reflect.TypeOf(SSHKey{}).Name()
	SSHKeyGroupKind        = schema.GroupKind{Group: Group, Kind: SSHKeyKind}.String()
	SSHKeyGroupVersionKind = SchemeGroupVersion.WithKind(SSHKeyKind)
)

func init() {
	SchemeBuilder.Register(&SSHKey{}, &SSHKeyList{})
}
