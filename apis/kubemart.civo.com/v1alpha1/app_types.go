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

	// The app name e.g. 'wordpress'
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// The action e.g. 'install' or 'update'
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=install;update
	Action string `json:"action,omitempty"`

	// The plan of the app (only for certain apps)
	// +kubebuilder:validation:Optional
	Plan string `json:"plan,omitempty"`
}

// JobInfo contains information about each Job we launched under an App
type JobInfo struct {
	// The last status of the Job e.g.
	// installation_started, installation_finished, installation_failed
	JobStatus string `json:"job_status"`

	// When the Job starts. Using pointer as a temp hack due to this issue:
	// https://github.com/kubernetes/kubernetes/issues/86811
	StartedAt *metav1.Time `json:"started_at,omitempty"`

	// When the Job ends. Using pointer as a temp hack due to this issue:
	// https://github.com/kubernetes/kubernetes/issues/86811
	EndedAt *metav1.Time `json:"ended_at,omitempty"`
}

// Configuration defines the app's configuration.
// For example, "mariadb" app will have "MYSQL_ROOT_PASSWORD" configuration.
type Configuration struct {
	// The key e.g. 'MYSQL_ROOT_PASSWORD'
	Key string `json:"key,omitempty"`

	// The value e.g. 'email@example.com'
	Value string `json:"value,omitempty"`

	// If the value was generated using 'KUBEMART:ALPHANUMERIC' or
	// 'KUBEMART:WORDS', this field will be 'true'. Default to 'false'.
	ValueIsBase64 bool `json:"value_is_base64,omitempty"`

	// Return true if the value was customized by "KUBEMART:***" variable.
	// For example, let's say this is the app manifest:
	// https://github.com/civo/kubernetes-marketplace/blob/6609b3bbe5857acae17dbf42f0f558feb84843e1/jenkins/manifest.yaml
	// The "JENKINS_USERNAME" and "JENKINS_PASSWORD" will have this attribute set to true.
	// But the "VOLUME_SIZE" will be false.
	IsCustomized bool `json:"is_customized,omitempty"`
}

// AppStatus defines the observed state of App
type AppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The installed version, retrieved from kubernetes-marketplace/app/manifest.yaml file
	InstalledVersion string `json:"installed_version,omitempty"`

	// The last Job's status
	LastStatus string `json:"last_status,omitempty"`

	// The last Job's name
	LastJobExecuted string `json:"last_job_executed,omitempty"`

	// Map of all Jobs that were executed for this App.
	// See docs for 'JobInfo' for more detail.
	JobsExecuted map[string]JobInfo `json:"jobs_executed,omitempty"`

	// All App's configurations.
	// See docs for 'Configuration' for more detail.
	Configurations []Configuration `json:"configurations,omitempty"`

	// Will return 'true' if installed app version doesn't match with
	// the latest version from kubernetes-marketplace repository.
	// Default to 'false'.
	NewUpdateAvailable bool `json:"new_update_available,omitempty"`

	// Will return the latest version from kubernetes-marketplace repository
	NewUpdateVersion string `json:"new_update_version,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="app"
// +kubebuilder:printcolumn:name="Current Status",type="string",JSONPath=".status.last_status",description="Latest status of the App"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.installed_version",description="Installed version of the App"
// +genclient
// k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
