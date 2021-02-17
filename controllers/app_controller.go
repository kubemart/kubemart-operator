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
	"reflect"
	"strings"
	"time"

	appv1alpha1 "github.com/civo/bizaar-operator/api/v1alpha1"
	"github.com/civo/bizaar-operator/pkg/utils"
	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// +kubebuilder:rbac:groups=bizaar.civo.com,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bizaar.civo.com,resources=apps/status,verbs=get;update;patch

const (
	// waiting period between "did all app's dependencies have been installed" checks
	pollIntervalSeconds       = 30
	updateWatcherSleepMinutes = 30
)

// Used to tell us if we have deployed update watcher for a given app.
// If the second return value from this map returns true,
// that means update watcher has been deployed.
var updateWatcher = make(map[string]bool)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// BizaarConfigMap is used when reading ConfigMap
type BizaarConfigMap struct {
	EmailAddress string
	DomainName   string
	ClusterName  string
	MasterIP     string
}

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
			return reconcile.Result{}, nil // stop reconcile
		}
		// Error reading the object
		return reconcile.Result{}, err // restart reconcile
	}

	lastStatus := appInstance.Status.LastStatus
	action := appInstance.Spec.Action

	finalizerName := "finalizers.bizaar.civo.com"
	if appInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object
		if !utils.ContainsString(appInstance.ObjectMeta.Finalizers, finalizerName) {
			appInstance.ObjectMeta.Finalizers = append(appInstance.ObjectMeta.Finalizers, finalizerName)
			err := r.Update(context.Background(), appInstance)
			if err != nil {
				return reconcile.Result{}, err // restart reconcile
			}
		}
	} else {
		// The object is being deleted
		// let's first clear the updateWatcher
		err := r.DeleteUpdateWatcher(appInstance)
		if err != nil {
			return reconcile.Result{}, err // restart reconcile
		}

		if utils.ContainsString(appInstance.ObjectMeta.Finalizers, finalizerName) {
			// our finalizer is present, so lets delete the app's namespace
			err := r.ProcessAppNamespaceDeletion(appInstance)
			if err != nil {
				// if fail to delete the namespace, return with error so that it can be retried
				return reconcile.Result{}, err // restart reconcile
			}

			// remove finalizer from the App
			appInstance.ObjectMeta.Finalizers = utils.RemoveString(appInstance.ObjectMeta.Finalizers, finalizerName)
			err = r.Update(context.Background(), appInstance)
			if err != nil {
				return reconcile.Result{}, err // restart reconcile
			}
		}

		// Stop reconciliation as the item is being deleted and does not have finalizer
		return ctrl.Result{}, nil
	}

	// For `bizaar install <app_name>`
	if lastStatus == "" && action == "install" {
		logger.Info("Entering pre-install stage")
		jobsExecuted := make(map[string]appv1alpha1.JobInfo)

		configMap, err := r.GetBizaarConfigMap()
		if err != nil {
			return reconcile.Result{}, err // restart reconcile
		}

		configurations, err := utils.GetAppConfigurations(appInstance.Name)
		if err != nil {
			return reconcile.Result{}, err // restart reconcile
		}

		configs := []appv1alpha1.Configuration{}
		for _, configuration := range configurations {
			configKey := configuration.Key
			configTemplate := configuration.Template

			justVariableName, err := utils.ExtractBizaarConfigTemplate(configTemplate)
			if err != nil {
				return reconcile.Result{}, err // restart reconcile
			}

			if strings.Contains(configTemplate, "BIZAAR:ALPHANUMERIC") {
				length, err := utils.ExtractNumFromBizaarConfigTemplate(configTemplate)
				if err != nil {
					return reconcile.Result{}, err // restart reconcile
				}

				randomChars, err := utils.GenerateRandomAlphanumeric(length, appInstance.Name, string(appInstance.UID))
				if err != nil {
					return reconcile.Result{}, err // restart reconcile
				}

				initialValue := strings.ReplaceAll(configTemplate, justVariableName, randomChars)
				base64Encoded := utils.GetBase64String(initialValue)
				configs = append(configs, appv1alpha1.Configuration{
					Key:           configKey,
					Value:         base64Encoded,
					ValueIsBase64: true,
				})
			}

			if strings.Contains(configTemplate, "BIZAAR:WORDS") {
				length, err := utils.ExtractNumFromBizaarConfigTemplate(configTemplate)
				if err != nil {
					return reconcile.Result{}, err // restart reconcile
				}

				randomWords := utils.GenerateRandomWords(length, appInstance.Name, string(appInstance.UID))
				initialValue := strings.ReplaceAll(configTemplate, justVariableName, randomWords)
				base64Encoded := utils.GetBase64String(initialValue)
				configs = append(configs, appv1alpha1.Configuration{
					Key:           configKey,
					Value:         base64Encoded,
					ValueIsBase64: true,
				})
			}

			if strings.Contains(configTemplate, "BIZAAR:CLUSTER_NAME") {
				clusterName := configMap.ClusterName
				value := strings.ReplaceAll(configTemplate, justVariableName, clusterName)
				configs = append(configs, appv1alpha1.Configuration{
					Key:   configKey,
					Value: value,
				})
			}

			if strings.Contains(configTemplate, "BIZAAR:DOMAIN_NAME") {
				domainName := configMap.DomainName
				value := strings.ReplaceAll(configTemplate, justVariableName, domainName)
				configs = append(configs, appv1alpha1.Configuration{
					Key:   configKey,
					Value: value,
				})
			}

			if strings.Contains(configTemplate, "BIZAAR:EMAIL_ADDRESS") {
				emailAddress := configMap.EmailAddress
				value := strings.ReplaceAll(configTemplate, justVariableName, emailAddress)
				configs = append(configs, appv1alpha1.Configuration{
					Key:   configKey,
					Value: value,
				})
			}

			if strings.Contains(configTemplate, "BIZAAR:MASTER_IP") {
				masterIP := configMap.MasterIP
				value := strings.ReplaceAll(configTemplate, justVariableName, masterIP)
				configs = append(configs, appv1alpha1.Configuration{
					Key:   configKey,
					Value: value,
				})
			}
		}

		// app plan
		appPlan := appInstance.Spec.Plan
		if appPlan > 0 {
			planVariableName, err := utils.GetAppPlanVariableName(appInstance.Spec.Name)
			if err != nil {
				return reconcile.Result{}, err // restart reconcile
			}
			configs = append(configs, appv1alpha1.Configuration{
				Key:   planVariableName,
				Value: fmt.Sprintf("%dGi", appPlan),
			})
		}

		appInstance.Status = appv1alpha1.AppStatus{
			LastStatus:     "pre_installation_started",
			JobsExecuted:   jobsExecuted,
			Configurations: configs,
		}
		err = r.Status().Update(ctx, appInstance)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return reconcile.Result{}, err // restart reconcile
		}

		// Race conditions safety net (maybe we can delete this)
		time.Sleep(3 * time.Second)
	}

	if lastStatus == "pre_installation_started" && action == "install" {
		logger.Info("Entering dependency install stage")
		installedApps, err := r.GetInstalledAppNamesMap()
		if err != nil {
			logger.Error(err, "Failed to get all installed apps")
			return reconcile.Result{}, err // restart reconcile
		}

		directDependencies, err := utils.GetAppDependencies(appInstance.Spec.Name)
		if err != nil {
			logger.Error(err, "Failed to get all dependencies for app", "app", appInstance.Spec.Name)
			return reconcile.Result{}, err // restart reconcile
		}

		depthDependencies := &[]string{}
		err = utils.GetDepthDependenciesToInstall(depthDependencies, directDependencies, installedApps)
		if err != nil {
			logger.Error(err, "Failed to get depth dependencies for app", "app", appInstance.Spec.Name)
			return reconcile.Result{}, err // restart reconcile
		}

		for _, depthDependency := range *depthDependencies {
			alreadyCreated := r.IsAppAlreadyCreated(depthDependency)
			if alreadyCreated {
				continue // do not install it again
			}

			depthDependencyPlans, err := utils.GetAppPlans(depthDependency)
			if err != nil {
				logger.Error(err, "Failed to get plans for app", "app", depthDependency)
				return reconcile.Result{}, err // restart reconcile
			}

			dApp := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name:      depthDependency,
					Namespace: "default",
				},
				Spec: appv1alpha1.AppSpec{
					Name:   depthDependency,
					Action: "install",
				},
			}

			if len(depthDependencyPlans) > 0 {
				dApp.Spec.Plan = utils.GetSmallestAppPlan(depthDependencyPlans)
			}

			err = r.Create(context.Background(), dApp, &client.CreateOptions{})
			if err != nil {
				logger.Error(err, "Failed to create dependency", "dependency name", depthDependency)
				return reconcile.Result{}, err // restart reconcile
			}
		}

		// Refresh the App instance first so we can update it
		err = r.Get(ctx, req.NamespacedName, appInstance)
		if err != nil {
			logger.Error(err, "Failed to refresh App instance")
			// Restart the Reconcile
			return reconcile.Result{}, err
		}
		status := appInstance.Status
		status.LastStatus = "dependencies_installation_started"
		appInstance.Status = status
		err = r.Status().Update(ctx, appInstance)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return reconcile.Result{}, err // restart reconcile
		}

	}

	if lastStatus == "dependencies_installation_started" && action == "install" {
		logger.Info("Entering install stage")

		dependencies, err := utils.GetAppDependencies(appInstance.Spec.Name)
		if err != nil {
			logger.Error(err, "Failed to get all dependencies for app", "app", appInstance.Spec.Name)
			return reconcile.Result{}, err // restart reconcile
		}

		if len(dependencies) > 0 {
			for _, dependency := range dependencies {
				dApp := &appv1alpha1.App{}
				err := r.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: dependency}, dApp)
				if err != nil {
					logger.Error(err, "Failed to get dependency", "name", dependency)
					return reconcile.Result{}, err // restart reconcile
				}
				// TODO - add apps in update status as well
				if dApp.Status.LastStatus != "installation_finished" {
					logger.Info("Waiting for dependency installation to complete", "app", appInstance.Spec.Name, "dependency", dependency)
					return ctrl.Result{RequeueAfter: pollIntervalSeconds * time.Second}, nil
				}
			}
		}

		jobsExecuted := make(map[string]appv1alpha1.JobInfo)
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
		jobsExecuted[job.Name] = jobInfo

		status := appInstance.Status
		status.LastStatus = jobStatus
		status.LastJobExecuted = job.Name
		status.JobsExecuted = jobsExecuted
		appInstance.Status = status
		err = r.Status().Update(ctx, appInstance)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return reconcile.Result{}, err // restart the reconcile
		}

		// Create JobWatcher for Job
		jobwatcher := &appv1alpha1.JobWatcher{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: appInstance.ObjectMeta.Name + "-jw-",
				Namespace:    "default",
			},
			Spec: appv1alpha1.JobWatcherSpec{
				Namespace:  "default",
				Frequency:  10, // check Job every 10 seconds
				AppName:    appInstance.ObjectMeta.Name,
				JobName:    job.Name,
				MaxRetries: 30,
			},
		}

		// Set App as the owner of the JobWatcher
		err = controllerutil.SetControllerReference(appInstance, jobwatcher, r.Scheme)
		if err != nil {
			return reconcile.Result{}, err // restart the reconcile
		}

		err = r.Create(context.Background(), jobwatcher, &client.CreateOptions{})
		if err != nil {
			logger.Error(err, "Failed to launch JobWatcher for app", "app", appInstance.Spec.Name)
			return reconcile.Result{}, err // restart the reconcile
		}
	}

	// TODO
	// Another reconcile loop to watch the Job progress
	// * If Job went OK, the lastStatus changed from `installation_started` to `installation_finished` and
	// * If Job failed, the lastStatus changed from `installation_started` to `installation_failed`

	if lastStatus == "installation_finished" && action == "update" {
		logger.Info("Entering update stage")
	}

	if lastStatus == "installation_finished" || lastStatus == "update_finished" {
		appName := appInstance.ObjectMeta.Name

		if appInstance.Status.InstalledVersion == "" {
			version, err := utils.GetAppVersion(appName)
			if err != nil {
				logger.Error(err, "Unable to get app's version")
				return reconcile.Result{}, nil // stop reconcile
			}

			appInstance.Status.InstalledVersion = version
			err = r.Status().Update(ctx, appInstance)
			if err != nil {
				logger.Error(err, "Failed to update status")
				return reconcile.Result{}, err // restart reconcile
			}
		}

		_, updateWatcherExists := updateWatcher[appName]
		if !updateWatcherExists {
			go r.WatchForNewUpdate(appInstance)
			updateWatcher[appName] = true
		}
	}

	return ctrl.Result{}, nil // stop reconcile
}

// WatchForNewUpdate ...
func (r *AppReconciler) WatchForNewUpdate(app *appv1alpha1.App) {
	logger := r.Log
	appName := app.ObjectMeta.Name
	logger.Info("Adding an update watcher for app", "app", appName)

	for {
		logger.Info("Checking for new update", "app", appName)
		versionFromManifest, err := utils.GetAppVersion(appName)
		if err != nil {
			logger.Error(err, "Unable to get app's version")
			continue // try again in next cycle
		}

		// refresh app instance
		err = r.Get(context.Background(), types.NamespacedName{
			Namespace: app.ObjectMeta.Namespace,
			Name:      app.ObjectMeta.Name,
		}, app)
		if err != nil {
			logger.Error(err, "Unable to refresh app instance")
			continue // try again in next cycle
		}

		installedVersion := app.Status.InstalledVersion
		if installedVersion != versionFromManifest {
			logger.Info("New update available", "app", appName, "installed", installedVersion, "available", versionFromManifest)
			app.Status.NewUpdateAvailable = true
			app.Status.NewUpdateVersion = versionFromManifest
		} else {
			logger.Info("Installed app is latest", "app", appName)
			app.Status.NewUpdateAvailable = false
			app.Status.NewUpdateVersion = ""
		}

		err = r.Status().Update(context.Background(), app, &client.UpdateOptions{})
		if err != nil {
			logger.Error(err, "Unable to update app status (new version available)")
			continue // try again in next cycle
		}

		// important to have this sleep, because this function will run forever
		// in background as Go routine
		time.Sleep(updateWatcherSleepMinutes * time.Minute)
	}
}

// DeleteUpdateWatcher ...
func (r *AppReconciler) DeleteUpdateWatcher(app *appv1alpha1.App) error {
	logger := r.Log
	appName := app.ObjectMeta.Name
	logger.Info("Deleting update watcher", "app", appName)

	_, exists := updateWatcher[appName]
	if exists {
		delete(updateWatcher, appName)
	}

	keys := reflect.ValueOf(updateWatcher).MapKeys()
	remainingUpdateWatcher := make([]string, len(keys))
	for i := 0; i < len(keys); i++ {
		remainingUpdateWatcher[i] = keys[i].String()
	}

	remainingWatchers := strings.Join(remainingUpdateWatcher, ",")
	logger.Info("Update watcher has been deleted", "remaining watcher", remainingWatchers)

	return nil
}

// ProcessAppNamespaceDeletion ...
func (r *AppReconciler) ProcessAppNamespaceDeletion(app *appv1alpha1.App) error {
	appName := app.ObjectMeta.Name
	logger := r.Log.WithValues("app", appName)

	logger.Info("Processing app's namespace deletion...")
	namespace, err := utils.GetNamespaceFromAppManifest(appName)
	if err != nil {
		logger.Error(err, "Unable to determine app namespace from app's manifest.yaml file")
		// stop here so the caller can continue with removing finalizer from Custom Resource
		return nil
	}

	if namespace == "" {
		logger.Info("Unable to delete namespace because this app does not have namespace declared in its manifest.yaml file")
		// stop here so the caller can continue with removing finalizer from Custom Resource
		return nil
	}

	namespaceExists, err := r.IsNamespaceExist(namespace)
	if err != nil {
		logger.Error(err, "Unable to check namespace")
		return err
	}

	if namespaceExists {
		logger.Info("Deleting namespace", "namespace", namespace)
		err = r.DeleteNamespace(namespace)
		if err != nil {
			logger.Error(err, "Unable to delete namespace")
			return err
		}
	}

	logger.Info("Waiting for namespace deletion to finish...", "namespace", namespace)
	var sleepDuration time.Duration = 5 // seconds
	var maxTries int = 60
	var tries int = 0
	var clearNamespaceFinalizer bool = false

	for {
		if tries > maxTries {
			clearNamespaceFinalizer = true
			break
		}

		nsExists, err := r.IsNamespaceExist(namespace)
		if err != nil {
			logger.Error(err, "Unable to check namespace")
			return err
		}

		if !nsExists {
			break
		}

		time.Sleep(sleepDuration * time.Second)
		tries++
	}

	if clearNamespaceFinalizer {
		logger.Info("Clearing namespace finalizer...")
		app.ObjectMeta.Finalizers = []string{}
		err := r.Update(context.Background(), app, &client.UpdateOptions{})
		if err != nil {
			logger.Error(err, "Unable to clear namespace finalizers", "namespace", namespace)
			return err
		}
	}

	return nil
}

// GetApps ...
func (r *AppReconciler) GetApps() (*appv1alpha1.AppList, error) {
	apps := &appv1alpha1.AppList{}
	err := r.List(context.Background(), apps, &client.ListOptions{})
	if err != nil {
		return apps, err
	}

	return apps, nil
}

// IsAppAlreadyCreated ...
func (r *AppReconciler) IsAppAlreadyCreated(appName string) bool {
	apps, err := r.GetApps()
	if err != nil {
		return false
	}

	for _, app := range apps.Items {
		if app.Spec.Name == appName {
			return true
		}
	}

	return false
}

// GetInstalledAppNamesMap ...
func (r *AppReconciler) GetInstalledAppNamesMap() (map[string]bool, error) {
	installedAppsMap := make(map[string]bool)

	apps, err := r.GetApps()
	if err != nil {
		return installedAppsMap, err
	}

	for _, app := range apps.Items {
		// TODO - add apps in update status as well
		if app.Status.LastStatus == "installation_finished" {
			installedAppsMap[app.Spec.Name] = true
		}
	}

	return installedAppsMap, nil
}

// GetBizaarConfigMap will fetch "bizaar-config" ConfigMap and returns BizaarConfigMap
func (r *AppReconciler) GetBizaarConfigMap() (*BizaarConfigMap, error) {
	bcm := &BizaarConfigMap{}
	configMap := &v1.ConfigMap{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name:      "bizaar-config",
		Namespace: "bizaar",
	}, configMap)
	if err != nil {
		return bcm, err
	}

	data := configMap.Data
	bcm.EmailAddress = data["email"]
	bcm.DomainName = data["domain"]
	bcm.ClusterName = data["cluster_name"]
	bcm.MasterIP = data["master_ip"]
	return bcm, nil
}

// IsNamespaceExist ...
func (r *AppReconciler) IsNamespaceExist(namespace string) (bool, error) {
	ns := &v1.Namespace{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name: namespace,
	}, ns)

	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// GetNamespace ...
func (r *AppReconciler) GetNamespace(namespace string) (*v1.Namespace, error) {
	ns := &v1.Namespace{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name: namespace,
	}, ns)

	if err != nil {
		return ns, err
	}

	return ns, nil
}

// DeleteNamespace ...
func (r *AppReconciler) DeleteNamespace(namespace string) error {
	ns, _ := r.GetNamespace(namespace)
	err := r.Client.Delete(context.Background(), ns, &client.DeleteOptions{})
	return err
}

// newJobPod is the pod definition that contains helm, kubectl, curl, git & Civo marketplace code
func newJobPod(cr *appv1alpha1.App) *batchv1.Job {
	// auto delete the Job (and its Pod) 24 hours after it finishes
	// https://kubernetes.io/docs/concepts/workloads/controllers/job/#ttl-mechanism-for-finished-jobs
	secondsInADay := int32(86400)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cr.Name + "-job-", // Job name example: app-sample-job-jzxbw
			Namespace:    cr.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &secondsInADay,
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
								fmt.Sprintf("./main --app-name %s --namespace default && cd scripts && ./install.sh", cr.Spec.Name),
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
