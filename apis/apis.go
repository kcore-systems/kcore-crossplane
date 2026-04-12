// Package apis registers Kubernetes API types for provider-kcore.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	kcorev1alpha1 "github.com/kcore/kcore-crossplane/apis/kcore/v1alpha1"
)

// AddToSchemes may be used to add all resources to a Scheme.
var AddToSchemes runtime.SchemeBuilder

func init() {
	AddToSchemes = append(AddToSchemes, kcorev1alpha1.SchemeBuilder.AddToScheme)
}

// AddToScheme adds all Resources to the Scheme.
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
