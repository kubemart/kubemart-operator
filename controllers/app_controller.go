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

	appv1alpha1 "github.com/civo/bizaar-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	// Fetch the App instance
	appInstance := &appv1alpha1.App{}
	err := r.Get(ctx, req.NamespacedName, appInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object
		return reconcile.Result{}, err
	}

	// When the CRD is new and there were no prior installation
	if appInstance.Status.NumOfJobs == 0 && appInstance.Status.LastStatus == "" {
		log.Info("---------------------------- LAUNCHING A NEW JOB ----------------------------")
		job := newJobPod(appInstance)
		// Set App instance as the owner of the Job
		if err := controllerutil.SetControllerReference(appInstance, job, r.Scheme); err != nil {
			return reconcile.Result{}, err
		}
		err = r.Create(context.Background(), job)
		// Failed to launch a new Job
		if err != nil {
			logger.Error(err, "Failed to create job", "job.name", job.Name)
			return reconcile.Result{}, err
		}
		// New Job was launched successfully
		logger.Info("Launched job", "job.name", job.Name)
		// Refresh the App instance first so we can update it
		err = r.Get(ctx, req.NamespacedName, appInstance)
		if err != nil {
			logger.Error(err, "Failed to refresh App instance")
			return reconcile.Result{}, err
		}
		numOfJobs := appInstance.Status.NumOfJobs + 1
		appInstance.Status = appv1alpha1.AppStatus{
			LastStatus: "RUNNING",
			NumOfJobs:  numOfJobs,
		}
		err = r.Status().Update(ctx, appInstance)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return reconcile.Result{}, err
		}
	}

	// When the controller created more than one active jobs due to concurrency
	desiredJobCount := 1
	if appInstance.Status.NumOfJobs > desiredJobCount && appInstance.Status.LastStatus == "RUNNING" {
		log.Info("---------------------------- SCALING DOWN JOBS ----------------------------")
		currentNum := appInstance.Status.NumOfJobs
		logger.Info("Scaling down jobs", "Current number", currentNum, "Desired number", desiredJobCount)
		diff := currentNum - desiredJobCount

		// List all jobs owned by this app instance
		jobList := &batchv1.JobList{}
		labelz := map[string]string{
			"bizaar-app-name":   appInstance.Spec.Name,
			"bizaar-job-status": "RUNNING",
		}
		labelSelector := labels.SelectorFromSet(labelz)
		listOpts := &client.ListOptions{Namespace: appInstance.Namespace, LabelSelector: labelSelector}
		if err = r.List(context.Background(), jobList, listOpts); err != nil {
			return reconcile.Result{}, err
		}

		// Delete extra jobs
		toDeleteJobs := jobList.Items[:diff]
		for _, toDeleteJob := range toDeleteJobs {
			fmt.Println("To delete job -->", toDeleteJob.Name)
			deletePropagationBg := metav1.DeletePropagationBackground
			err = r.Delete(context.Background(), &toDeleteJob, &client.DeleteOptions{
				PropagationPolicy: &deletePropagationBg,
			})
			if err != nil {
				r.Log.Error(err, "Failed to delete job", "job.name", toDeleteJob.Name)
				return reconcile.Result{}, err
			}
			// Refresh the App instance first so we can update it
			err = r.Get(ctx, req.NamespacedName, appInstance)
			if err != nil {
				logger.Error(err, "Failed to refresh App instance")
				return reconcile.Result{}, err
			}
			numOfJobs := appInstance.Status.NumOfJobs - 1
			appInstance.Status = appv1alpha1.AppStatus{
				LastStatus: "RUNNING",
				NumOfJobs:  numOfJobs,
			}
			err = r.Status().Update(ctx, appInstance)
			if err != nil {
				logger.Error(err, "Failed to update status")
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{Requeue: true}, nil
	}

	// End of reconcile logic
	return ctrl.Result{}, nil
}

// newJobPod is the pod definition that contains helm, kubectl, curl, git & Civo marketplace code
func newJobPod(cr *appv1alpha1.App) *batchv1.Job {
	labels := map[string]string{
		"bizaar-app-name":   cr.Spec.Name,
		"bizaar-job-status": "RUNNING",
	}
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cr.Name + "-job-", // Job name example: app-sample-job-jzxbw
			Namespace:    cr.Namespace,
			Labels:       labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{

					RestartPolicy: "OnFailure",
					Containers: []corev1.Container{
						{
							Name:    "bizaar",
							Image:   "ubuntu",                  // TODO - change to "civo/image:tag"
							Command: []string{"sleep", "3600"}, // TODO - replace with helm/kubectl commands
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

// TODO
// - After the Job is complete, update `LastStatus` to `COMPLETED`
