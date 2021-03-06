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

	Name   string `json:"name,omitempty"` // app name i.e. wordpress
	Action string `json:"action,omitempty"`
	Plan   int    `json:"plan,omitempty"`
}

// JobInfo contains information about each Job we launched under an App
type JobInfo struct {
	JobStatus string `json:"job_status"`

	// Using pointer as a temp hack due to this issue:
	// https://github.com/kubernetes/kubernetes/issues/86811
	StartedAt *metav1.Time `json:"started_at,omitempty"`
	EndedAt   *metav1.Time `json:"ended_at,omitempty"`
}

// Configuration defines the app's configuration.
// For example, "mariadb" app will have "MYSQL_ROOT_PASSWORD" configuration.
type Configuration struct {
	Key           string `json:"key,omitempty"`
	Value         string `json:"value,omitempty"`
	ValueIsBase64 bool   `json:"value_is_base64,omitempty"`
}

// AppStatus defines the observed state of App
type AppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	InstalledVersion   string             `json:"installed_version,omitempty"`
	LastStatus         string             `json:"last_status,omitempty"`
	LastJobExecuted    string             `json:"last_job_executed,omitempty"`
	JobsExecuted       map[string]JobInfo `json:"jobs_executed,omitempty"`
	Configurations     []Configuration    `json:"configurations,omitempty"`
	NewUpdateAvailable bool               `json:"new_update_available,omitempty"`
	NewUpdateVersion   string             `json:"new_update_version,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="app"
// +genclient

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
