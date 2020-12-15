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

package controllers

import (
	"context"
	"fmt"
	"time"

	appv1alpha1 "github.com/civo/bizaar-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=app.bizaar.civo.com,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bizaar.civo.com,resources=apps/status,verbs=get;update;patch

// Reconcile is called either when one of our CRDs change
// or if the returned ctrl.Result isn’t empty (or an error is returned)
func (r *AppReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log.WithValues("app", req.NamespacedName)
	logger.Info("App Reconcile started...")

	// Fetch the App instance
	appInstance := &appv1alpha1.App{}
	err := r.Get(ctx, req.NamespacedName, appInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Stop the Reconcile
			return reconcile.Result{}, nil
		}
		// Error reading the object. Restart the Reconcile.
		return reconcile.Result{}, err
	}

	lastStatus := appInstance.Status.LastStatus
	targetStatus := appInstance.Spec.TargetStatus

	// For `bizaar install <app_name>`
	if lastStatus == "" && targetStatus == "installed" {
		logger.Info("Installing new app...")
		job := newJobPod(appInstance)
		// Set App instance as the owner of the Job
		if err := controllerutil.SetControllerReference(appInstance, job, r.Scheme); err != nil {
			// Restart the Reconcile
			return reconcile.Result{}, err
		}
		// Create a Job
		err = r.Create(context.Background(), job)
		if err != nil {
			logger.Error(err, "Failed to create job", "job.name", job.Name)
			// Restart the Reconcile
			return reconcile.Result{}, err
		}
		// New Job was launched successfully
		logger.Info("New Job was launched successfully", "job.name", job.Name)
		// Refresh the App instance first so we can update it
		err = r.Get(ctx, req.NamespacedName, appInstance)
		if err != nil {
			logger.Error(err, "Failed to refresh App instance")
			// Restart the Reconcile
			return reconcile.Result{}, err
		}
		jobStatus := "installation_started"
		timeNow := metav1.Now()
		jobInfo := appv1alpha1.JobInfo{
			JobStatus: jobStatus,
			StartedAt: &timeNow,
		}
		jobsExecuted := make(map[string]appv1alpha1.JobInfo)
		jobsExecuted[job.Name] = jobInfo
		appInstance.Status = appv1alpha1.AppStatus{
			LastStatus:      jobStatus,
			LastJobExecuted: job.Name,
			JobsExecuted:    jobsExecuted,
		}
		err = r.Status().Update(ctx, appInstance)
		if err != nil {
			logger.Error(err, "Failed to update status")
			// Restart the Reconcile
			return reconcile.Result{}, err
		}
		// Race conditions safety net
		time.Sleep(3 * time.Second)
		// Stop the Reconcile
		return ctrl.Result{}, nil
	}

	// TODO
	// Another reconcile loop to watch the Job progress
	// * If Job went OK, the lastStatus changed from `installation_started` to `installation_finished` and
	// * If Job failed, the lastStatus changed from `installation_started` to `installation_failed`
	// Both cases should change `targetStatus` to "" (empty string) in the CRD YAML file

	if lastStatus == "installation_finished" && targetStatus == "updated" {
		logger.Info("Updating app...")
		// Stop the Reconcile
		return ctrl.Result{}, nil
	}

	if lastStatus == "installation_finished" && targetStatus == "deleted" {
		logger.Info("Deleting app...")
		// Stop the Reconcile
		return ctrl.Result{}, nil
	}

	logger.Info("No action needed", "LastStatus", lastStatus, "TargetStatus", targetStatus)
	// Stop the Reconcile
	return ctrl.Result{}, nil
}

// newJobPod is the pod definition that contains helm, kubectl, curl, git & Civo marketplace code
func newJobPod(cr *appv1alpha1.App) *batchv1.Job {
	// labels := map[string]string{
	// 	"bizaar-app-name":   cr.Spec.Name,
	// 	"bizaar-job-status": "RUNNING",
	// }
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cr.Name + "-job-", // Job name example: app-sample-job-jzxbw
			Namespace:    cr.Namespace,
			// Labels:       labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "bizaar-daemon-svc-acc",
					RestartPolicy:      "OnFailure",
					Containers: []corev1.Container{
						{
							Name:            "bizaar-daemon",
							Image:           "civo/bizaar-daemon:v1alpha1",
							ImagePullPolicy: corev1.PullAlways,
							Command: []string{
								"/bin/sh", "-c",
							},
							Args: []string{
								// Note:
								// The `--namespace` is the namespace where the App custom resource is running.
								// Not where the actual workload i.e. wordpress is running.
								fmt.Sprintf("./main --app-name %s --crd-app-name %s --namespace default && cd scripts && ./run.sh", cr.Spec.Name, cr.Name),
							},
						},
					},
				},
			},
		},
	}
}

// SetupWithManager defines how the controller will watch for resources
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.App{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
