package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// SecurityGroupAttachmentParameters map to AttachSecurityGroupRequest.
type SecurityGroupAttachmentParameters struct {
	SecurityGroup string `json:"securityGroup"`
	// TargetKind is "vm" or "network".
	TargetKind string `json:"targetKind"`
	TargetID   string `json:"targetId"`
	TargetNode string `json:"targetNode,omitempty"`
}

// SecurityGroupAttachmentObservation is minimal (attachment has no separate id).
type SecurityGroupAttachmentObservation struct {
	Attached bool `json:"attached,omitempty"`
}

// SecurityGroupAttachmentSpec defines desired state.
type SecurityGroupAttachmentSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              SecurityGroupAttachmentParameters `json:"forProvider"`
}

// SecurityGroupAttachmentStatus is observed state.
type SecurityGroupAttachmentStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SecurityGroupAttachmentObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,kcore}
// SecurityGroupAttachment attaches a security group to a VM or network.
type SecurityGroupAttachment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityGroupAttachmentSpec   `json:"spec"`
	Status SecurityGroupAttachmentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// SecurityGroupAttachmentList contains attachments.
type SecurityGroupAttachmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityGroupAttachment `json:"items"`
}

var (
	SecurityGroupAttachmentKind             = reflect.TypeOf(SecurityGroupAttachment{}).Name()
	SecurityGroupAttachmentGroupKind        = schema.GroupKind{Group: Group, Kind: SecurityGroupAttachmentKind}.String()
	SecurityGroupAttachmentGroupVersionKind = SchemeGroupVersion.WithKind(SecurityGroupAttachmentKind)
)

func init() {
	SchemeBuilder.Register(&SecurityGroupAttachment{}, &SecurityGroupAttachmentList{})
}
