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

/*
Inspired from:
- https://link.medium.com/keqwXlFt4bb
- https://git.io/JIzTn
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kbatch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/kubemart/kubemart-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// JobWatcherReconciler reconciles a JobWatcher object
type JobWatcherReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=kubemart.civo.com,resources=jobwatchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubemart.civo.com,resources=jobwatchers/status,verbs=get;update;patch

// Reconcile is called either when one of our CRDs changed
// or if the returned ctrl.Result isn’t empty (or an error is returned)
func (r *JobWatcherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("JobWatcher", req.NamespacedName)

	watcher := &appv1alpha1.JobWatcher{}
	err := r.Get(ctx, req.NamespacedName, watcher)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil // stop reconcile
		}
		// Error reading the object
		return reconcile.Result{}, err // restart reconcile
	}

	underMaxRetries := watcher.Status.CurrentAttempt <= watcher.Spec.MaxRetries
	if underMaxRetries && !watcher.Status.Reconciled {
		if err := r.checkJob(ctx, watcher); err != nil {
			log.Error(err, "Unable to process JobWatcher")
		}

		return ctrl.Result{RequeueAfter: time.Duration(watcher.Spec.Frequency) * time.Second}, nil
	}

	return ctrl.Result{}, nil // stop reconcile
}

// checkJob will check for Job's conditions and trigger status update for App
func (r *JobWatcherReconciler) checkJob(ctx context.Context, watcher *appv1alpha1.JobWatcher) error {
	log := r.Log.WithValues("JobWatcher", watcher.Namespace+"/"+watcher.Name)
	r.Recorder.Event(watcher, "Normal", "CheckJobStarted", "JobWatcher is checking if the Job is complete")

	jobName := watcher.Spec.JobName
	jobNamespace := watcher.Spec.Namespace

	job := &kbatch.Job{}
	err := r.Get(context.Background(), types.NamespacedName{Namespace: jobNamespace, Name: jobName}, job)
	if err != nil {
		log.Error(err, "Unable to get jobs", "job namespace", jobNamespace, "job name", jobName)
		return err
	}

	for _, condition := range job.Status.Conditions {
		if condition.Status == corev1.ConditionTrue {
			if condition.Type == kbatch.JobComplete {
				jobStatus := "installation_finished"
				return r.updateAppAndWatcherStatus(jobStatus, watcher)
			}
			if condition.Type == kbatch.JobFailed {
				jobStatus := "installation_failed"
				return r.updateAppAndWatcherStatus(jobStatus, watcher)
			}
		}
	}

	return nil
}

// updateAppAndWatcherStatus will update App's status when the launcher Job has completed
// launching the kubemart-daemon pod (to install the actual app). It will also update JobWatcher status.
func (r *JobWatcherReconciler) updateAppAndWatcherStatus(jobStatus string, watcher *appv1alpha1.JobWatcher) error {
	log := r.Log.WithValues("JobWatcher", watcher.Namespace+"/"+watcher.Name)
	ctx := context.Background()
	log.Info("Updating app status", "status", jobStatus)
	r.Recorder.Event(watcher, "Normal", "UpdateAppAndJobWatcherStarted", fmt.Sprintf("Updating App's LastStatus field to %s", jobStatus))

	jobName := watcher.Spec.JobName
	appNamespacedName := types.NamespacedName{
		Name:      watcher.Spec.AppName,
		Namespace: watcher.Spec.Namespace, // App and JobWatcher are in the same namespace
	}

	// Fetch App
	app := &appv1alpha1.App{}
	err := r.Get(ctx, appNamespacedName, app)
	if err != nil {
		return err
	}

	// Update App status
	timeNow := metav1.Now()
	jobInfo := app.Status.JobsExecuted[jobName]
	jobInfo.JobStatus = jobStatus
	jobInfo.EndedAt = &timeNow

	app.Status.LastStatus = jobStatus
	app.Status.JobsExecuted[jobName] = jobInfo

	err = r.Status().Update(ctx, app)
	if err != nil {
		return err
	}

	// Fetch watcher
	jw := &appv1alpha1.JobWatcher{}
	jwNamespacedName := types.NamespacedName{
		Namespace: watcher.ObjectMeta.Namespace,
		Name:      watcher.ObjectMeta.Name,
	}
	err = r.Get(ctx, jwNamespacedName, jw)
	if err != nil {
		return err
	}

	// Update watcher
	jw.Status.CurrentAttempt++
	jw.Status.JobStatus = jobStatus
	jw.Status.Reconciled = true
	err = r.Status().Update(ctx, jw)
	if err != nil {
		return err
	}

	r.Recorder.Event(watcher, "Normal", "UpdateAppAndJobWatcherFinished", "App and JobWatcher are successfully updated")
	return nil
}

// SetupWithManager defines how the controller will watch for resources
func (r *JobWatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.JobWatcher{}).
		Complete(r)
}
