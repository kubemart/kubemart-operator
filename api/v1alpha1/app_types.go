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

// AppSpec defines the desired state of App
type AppSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	GitURL           string `json:"giturl,omitempty"`
	GitHash          string `json:"githash,omitempty"`
	GitFile          string `json:"gitfile,omitempty"` // main tracking file (to be used for `bizaar app update`)
	Name             string `json:"name,omitempty"`    // app name i.e. wordpress
	TargetStatus     string `json:"targetstatus,omitempty"`
	HelmReleaseName  string `json:"helmreleasename,omitempty"`
	HelmMetadataName string `json:"helmmetadataname,omitempty"`
}

// JobInfo contains information about each Job we launched under an App
type JobInfo struct {
	JobStatus string `json:"jobstatus"`

	// Using pointer as a temp hack due to this issue:
	// https://github.com/kubernetes/kubernetes/issues/86811
	StartedAt *metav1.Time `json:"startedat,omitempty"`
	EndedAt   *metav1.Time `json:"endedat,omitempty"`
}

// AppStatus defines the observed state of App
type AppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	LastStatus      string             `json:"laststatus"`
	LastJobExecuted string             `json:"lastjobexecuted"`
	JobsExecuted    map[string]JobInfo `json:"jobsexecuted"`
	// JobFailureMessage string   `json:"jobfailuremessage"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// App is the Schema for the apps API
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppList contains a list of App
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}
