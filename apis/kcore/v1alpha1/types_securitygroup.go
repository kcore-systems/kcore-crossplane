package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// SecurityGroupRule mirrors proto SecurityGroupRule.
type SecurityGroupRule struct {
	ID         string `json:"id,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
	HostPort   int32  `json:"hostPort,omitempty"`
	TargetPort int32  `json:"targetPort,omitempty"`
	SourceCidr string `json:"sourceCidr,omitempty"`
	TargetVM   string `json:"targetVm,omitempty"`
	EnableDnat bool   `json:"enableDnat,omitempty"`
}

// SecurityGroupParameters map to CreateSecurityGroupRequest.
type SecurityGroupParameters struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Rules       []SecurityGroupRule `json:"rules,omitempty"`
}

// SecurityGroupObservation captures remote state.
type SecurityGroupObservation struct {
	Name string `json:"name,omitempty"`
}

// SecurityGroupSpec defines desired state.
type SecurityGroupSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              SecurityGroupParameters `json:"forProvider"`
}

// SecurityGroupStatus is observed state.
type SecurityGroupStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          SecurityGroupObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,kcore}
// SecurityGroup manages kcore security groups.
type SecurityGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityGroupSpec   `json:"spec"`
	Status SecurityGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// SecurityGroupList contains SecurityGroup objects.
type SecurityGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityGroup `json:"items"`
}

var (
	SecurityGroupKind             = reflect.TypeOf(SecurityGroup{}).Name()
	SecurityGroupGroupKind        = schema.GroupKind{Group: Group, Kind: SecurityGroupKind}.String()
	SecurityGroupGroupVersionKind = SchemeGroupVersion.WithKind(SecurityGroupKind)
)

func init() {
	SchemeBuilder.Register(&SecurityGroup{}, &SecurityGroupList{})
}
