/*


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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// JobWatcherSpec defines the desired state of JobWatcher
type JobWatcherSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Namespace to watch
	Namespace string `json:"namespace"`

	// +kubebuilder:validation:Minimum=10
	// Frequency of the TTL checks
	Frequency int64 `json:"frequency"`

	// The metadata.name of App CRD that launched this JobWatcher
	AppName string `json:"app_name"`

	// Job (Bizaar Daemon) name
	JobName string `json:"job_name"`

	// +kubebuilder:validation:Minimum=10
	MaxRetries int64 `json:"max_retries"`
}

// JobWatcherStatus defines the observed state of JobWatcher
type JobWatcherStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The status of the Job that JobWatcher is watching
	JobStatus string `json:"job_status,omitempty"`

	// If we have updated App's status, Reconciled will be true
	Reconciled bool `json:"reconciled,omitempty"`

	// This will get bumped after every job check cycle
	CurrentAttempt int64 `json:"current_attempt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="jw"

// JobWatcher is the Schema for the jobwatchers API
type JobWatcher struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JobWatcherSpec   `json:"spec,omitempty"`
	Status JobWatcherStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// JobWatcherList contains a list of JobWatcher
type JobWatcherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JobWatcher `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JobWatcher{}, &JobWatcherList{})
}
