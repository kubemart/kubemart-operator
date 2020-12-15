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
	"regexp"
	"time"

	"github.com/go-logr/logr"
	kbatch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/civo/bizaar-operator/api/v1alpha1"
	batchv1alpha1 "github.com/civo/bizaar-operator/api/v1alpha1"
)

// JobWatcherReconciler reconciles a JobWatcher object
type JobWatcherReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// The metadata.name of App CRD that launched this JobWatcher
var watchFor string

// +kubebuilder:rbac:groups=batch.esys.github.com,resources=jobwatchers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.esys.github.com,resources=jobwatchers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch.esys.github.com,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.esys.github.com,resources=cronjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// Reconcile is called either when one of our CRDs change
// or if the returned ctrl.Result isn’t empty (or an error is returned)
func (r *JobWatcherReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("JobWatcher", req.NamespacedName)
	log.Info("JobWatcher Reconcile started...")

	var watcher batchv1alpha1.JobWatcher
	if err := r.Get(ctx, req.NamespacedName, &watcher); err != nil {
		log.Error(err, "Unable to fetch JobWatcher", "Request", req)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	watchFor = watcher.Spec.WatchFor
	watcher.Status.LastStarted = metav1.Time{Time: time.Now()}

	// Fetch all namespaces in the cluster
	var namespaces corev1.NamespaceList
	if err := r.List(ctx, &namespaces); err != nil {
		log.Error(err, "Unable to list Namespaces")
		return ctrl.Result{}, err
	}

	for _, ns := range namespaces.Items {
		if err := r.processNamespace(ctx, watcher, ns); err != nil {
			r.Log.Error(err, "Unable to process JobWatcher", "Patterns", watcher.Spec.NamespacePatterns)
		}
	}

	watcher.Status.LastFinished = metav1.Time{Time: time.Now()}

	if err := r.Status().Update(ctx, &watcher); err != nil {
		log.Error(err, "Unable to update JobWatcher status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Duration(watcher.Spec.Frequency) * time.Second}, nil
}

// If passed namespace matched with the namespace that we're observing, process the Jobs inside it
func (r *JobWatcherReconciler) processNamespace(ctx context.Context, watcher batchv1alpha1.JobWatcher, ns corev1.Namespace) error {
	log := r.Log.WithValues("JobWatcher", watcher.Namespace+"/"+watcher.Name, "Namespace", ns.Name)
	nsPatterns := watcher.Spec.NamespacePatterns
	match, err := matchCachedRegex(ns.Name, nsPatterns)
	if err != nil {
		log.Error(err, "Unable to compile spec job name as regex", "Patterns", nsPatterns)
	}
	if !match {
		return nil
	}

	r.Log.V(1).Info("Finding jobs for namespace", "Namespace", ns.Name)
	var jobs kbatch.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(ns.Name)); err != nil {
		log.Error(err, "Unable to list Jobs", "Namespace", ns)
		return err
	}

	for _, job := range jobs.Items {
		for _, c := range job.Status.Conditions {
			if c.Status == corev1.ConditionTrue {
				if c.Type == kbatch.JobComplete {
					// log.Info("------------------ JOB COMPLETE ------------------")
					return r.updateJobStatus(job.Name, true, job.Namespace)
				}
				if c.Type == kbatch.JobFailed {
					// log.Info("------------------- JOB FAILED -------------------")
					return r.updateJobStatus(job.Name, false, job.Namespace)
				}
			}
		}
	}

	return nil
}

func (r *JobWatcherReconciler) updateJobStatus(jobName string, isCompleted bool, namespace string) error {
	ctx := context.Background()
	ns := types.NamespacedName{
		Namespace: namespace,
		Name:      watchFor,
	}

	// Define Job status
	var jobStatus string
	if isCompleted {
		jobStatus = "installation_finished"
	} else {
		jobStatus = "installation_failed"
	}

	// Fetch the App instance
	appInstance := &appv1alpha1.App{}
	err := r.Get(ctx, ns, appInstance)
	if err != nil {
		return err
	}

	// Update App status
	timeNow := metav1.Now()
	jobInfo := appInstance.Status.JobsExecuted[jobName]
	jobInfo.JobStatus = jobStatus
	jobInfo.EndedAt = &timeNow
	jobsExecuted := make(map[string]appv1alpha1.JobInfo)
	jobsExecuted[jobName] = jobInfo
	appInstance.Status.LastStatus = jobStatus
	appInstance.Status.JobsExecuted = jobsExecuted
	err = r.Status().Update(ctx, appInstance)
	if err != nil {
		return err
	}

	// Update App spec.targetstatus to empty string
	appInstance.Spec.TargetStatus = ""
	err = r.Update(ctx, appInstance)
	if err != nil {
		return err
	}

	return nil
}

var compiledRegex = make(map[string]*regexp.Regexp)

func matchCachedRegex(s string, exprs []string) (bool, error) {
	for _, expr := range exprs {
		if _, ok := compiledRegex[expr]; !ok {
			regex, err := regexp.Compile(expr)
			if err != nil {
				return false, err
			}
			compiledRegex[expr] = regex
		}
		if compiledRegex[expr].MatchString(s) {
			return true, nil
		}
	}
	return false, nil
}

// SetupWithManager defines how the controller will watch for resources
func (r *JobWatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1alpha1.JobWatcher{}).
		Complete(r)
}
