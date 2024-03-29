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

	"github.com/go-logr/logr"
	"github.com/kubemart/kubemart-operator/apis/kubemart.civo.com/v1alpha1"
	appv1alpha1 "github.com/kubemart/kubemart-operator/apis/kubemart.civo.com/v1alpha1"
	"github.com/kubemart/kubemart-operator/pkg/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubemart.civo.com,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubemart.civo.com,resources=apps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update

const (
	// waiting period between "did all app's dependencies have been installed" checks
	pollIntervalSeconds       = 30
	updateWatcherSleepMinutes = 5 // TODO - change this to 30 before we go live
)

// Used to tell us if we have deployed update watcher for a given app.
// If the second return value from this map returns true,
// that means update watcher has been deployed.
var updateWatcher = make(map[string]bool)

// Used in App deletion process
var finalizerName = "finalizers.kubemart.civo.com"

// Terminating apps will be added into this map and released when the
// namespace is deleted
var terminatingApps = make(map[string]bool)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Log      logr.Logger
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// KubemartConfigMap is used when reading ConfigMap
type KubemartConfigMap struct {
	EmailAddress string
	DomainName   string
	ClusterName  string
	MasterIP     string
}

// Reconcile is called either when one of our CRDs changed
// or if the returned ctrl.Result isn’t empty (or an error is returned)
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

		// avoid reconciler from rerun this block again and again while waiting for app's
		// namespace to get terminated
		_, isTerminating := terminatingApps[appInstance.ObjectMeta.Name]
		if isTerminating {
			return ctrl.Result{}, nil // stop reconcile
		}

		terminatingApps[appInstance.ObjectMeta.Name] = true

		// add an event
		r.Recorder.Event(appInstance, "Normal", "UninstallStarted", "App is entering uninstall stage")

		// let's first clear the updateWatcher
		err = r.DeleteUpdateWatcher(appInstance)
		if err != nil {
			return reconcile.Result{}, err // restart reconcile
		}

		if utils.ContainsString(appInstance.ObjectMeta.Finalizers, finalizerName) {
			// execute app's uninstall.sh bash script - if the file is present
			uninstallJob := newJobPod(appInstance, true)
			// set App instance as the owner of the Job
			if err := controllerutil.SetControllerReference(appInstance, uninstallJob, r.Scheme); err != nil {
				// Restart the Reconcile
				return reconcile.Result{}, err
			}

			// create the Job
			err = r.Create(context.Background(), uninstallJob)
			if err != nil {
				logger.Error(err, "Failed to create uninstall job", "job name", uninstallJob.Name)
				// restart the Reconcile
				return reconcile.Result{}, err
			}
			logger.Info("New uninstall Job was launched successfully", "job name", uninstallJob.Name)
			logger.Info("Waiting for uninstall Job to complete", "job name", uninstallJob.Name)

			// Perform app's namespace deletion in goroutine/background so it won't block the reconcile loop.
			// For example, new app(s) will able to get reconciled without having to wait for the namespace deletion
			// to fully complete.
			go func() {
				// Wait until uninstall.sh job completes
				currentTry := 0
				maxTries := 60
				for {
					currentTry++
					if currentTry > maxTries {
						logger.Info("Max tries reached when checking uninstall.sh job status", "job name", uninstallJob.Name)
						break
					}

					uj := &batchv1.Job{}
					_ = r.Get(context.Background(), types.NamespacedName{
						Name:      uninstallJob.Name,
						Namespace: uninstallJob.Namespace,
					}, uj)

					if !uj.Status.CompletionTime.IsZero() {
						logger.Info("Job has completed, we are good to delete the app's namespace", "job name", uninstallJob.Name)
						break
					}

					time.Sleep(1 * time.Second)
				}

				// Delete the namespace
				r.ProcessAppNamespaceDeletion(appInstance)
			}()
		}

		// Stop reconciliation as the item is being deleted and does not have finalizer
		return ctrl.Result{}, nil
	}

	// When user want to update an application, we will clear installed_version and
	// last_status fields before "re-install" the app (with latest version)
	if action == "update" {
		logger.Info("Setting up update operation...")
		r.Recorder.Event(appInstance, "Normal", "UpdateStarted", "App is entering update stage")

		appInstance.Status.InstalledVersion = ""
		appInstance.Status.LastStatus = ""
		appInstance.Status.NewUpdateAvailable = false
		appInstance.Status.NewUpdateVersion = ""
		_ = r.Status().Update(context.Background(), appInstance)

		// because we just need to know if "it's an update" once,
		// let's update it to original value
		appInstance.Spec.Action = "install"
		err = r.Update(context.Background(), appInstance, &client.UpdateOptions{})
		if err != nil {
			logger.Error(err, "Failed to update app .spec.action field")
			return reconcile.Result{}, err // restart reconcile
		}

		// it's important to restart reconcile here so next cycle
		// will enter the next block (pre-install stage)
		time.Sleep(3 * time.Second)
		return ctrl.Result{Requeue: true}, nil
	}

	// For `kubemart install <app_name>`
	if lastStatus == "" {
		logger.Info("Entering pre-install stage")
		r.Recorder.Event(appInstance, "Normal", "PreInstallStarted", "App is entering pre-install stage")

		configMap, err := r.GetKubemartConfigMap()
		if err != nil {
			return reconcile.Result{}, err // restart reconcile
		}

		configurations, err := utils.GetAppConfigurations(appInstance.Name)
		if err != nil {
			return reconcile.Result{}, err // restart reconcile
		}

		configs := appInstance.Status.Configurations
		for _, configuration := range configurations {
			alreadyCreated := r.IsConfigurationExists(appInstance, configuration.Key)
			if alreadyCreated {
				continue
			}

			configKey := configuration.Key
			configTemplate := configuration.Template

			justVariableName, err := utils.ExtractKubemartConfigTemplate(configTemplate)
			if err != nil {
				return reconcile.Result{}, err // restart reconcile
			}

			if strings.Contains(configTemplate, "KUBEMART:ALPHANUMERIC") {
				length, err := utils.ExtractNumFromKubemartConfigTemplate(configTemplate)
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
					IsCustomized:  true,
				})
			}

			if strings.Contains(configTemplate, "KUBEMART:WORDS") {
				length, err := utils.ExtractNumFromKubemartConfigTemplate(configTemplate)
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
					IsCustomized:  true,
				})
			}

			if strings.Contains(configTemplate, "KUBEMART:CLUSTER_NAME") {
				clusterName := configMap.ClusterName
				value := strings.ReplaceAll(configTemplate, justVariableName, clusterName)
				configs = append(configs, appv1alpha1.Configuration{
					Key:           configKey,
					Value:         value,
					ValueIsBase64: false,
					IsCustomized:  true,
				})
			}

			if strings.Contains(configTemplate, "KUBEMART:DOMAIN_NAME") {
				domainName := configMap.DomainName
				value := strings.ReplaceAll(configTemplate, justVariableName, domainName)
				configs = append(configs, appv1alpha1.Configuration{
					Key:           configKey,
					Value:         value,
					ValueIsBase64: false,
					IsCustomized:  true,
				})
			}

			if strings.Contains(configTemplate, "KUBEMART:EMAIL_ADDRESS") {
				emailAddress := configMap.EmailAddress
				value := strings.ReplaceAll(configTemplate, justVariableName, emailAddress)
				configs = append(configs, appv1alpha1.Configuration{
					Key:           configKey,
					Value:         value,
					ValueIsBase64: false,
					IsCustomized:  true,
				})
			}

			if strings.Contains(configTemplate, "KUBEMART:MASTER_IP") {
				masterIP := configMap.MasterIP
				value := strings.ReplaceAll(configTemplate, justVariableName, masterIP)
				configs = append(configs, appv1alpha1.Configuration{
					Key:           configKey,
					Value:         value,
					ValueIsBase64: false,
					IsCustomized:  true,
				})
			}
		}

		// app plan
		appPlan := appInstance.Spec.Plan
		if appPlan != "" {
			planVariableName, err := utils.GetAppPlanVariableName(appInstance.Spec.Name)
			if err != nil {
				return reconcile.Result{}, err // restart reconcile
			}

			planExists := r.IsConfigurationExists(appInstance, planVariableName)
			if !planExists {
				configs = append(configs, appv1alpha1.Configuration{
					Key:           planVariableName,
					Value:         appPlan,
					ValueIsBase64: false,
					IsCustomized:  false,
				})
			}
		}

		appInstance.Status.LastStatus = "pre_installation_started"
		appInstance.Status.Configurations = configs
		err = r.Status().Update(ctx, appInstance)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return reconcile.Result{}, err // restart reconcile
		}

		time.Sleep(3 * time.Second)
		return ctrl.Result{}, nil // stop reconcile
	}

	if lastStatus == "pre_installation_started" {
		logger.Info("Entering dependency install stage")
		r.Recorder.Event(appInstance, "Normal", "DependencyInstallStarted", "App is entering dependency install stage")

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
					Namespace: "kubemart-system",
				},
				Spec: appv1alpha1.AppSpec{
					Name:   depthDependency,
					Action: "install",
				},
			}

			if len(depthDependencyPlans) > 0 {
				smallestPlanLabel := utils.GetSmallestAppPlan(depthDependencyPlans)
				smallestPlanValue, err := utils.GetAppPlanValueByLabel(depthDependency, smallestPlanLabel)
				if err != nil {
					logger.Error(err, "Failed to get the smallest plan value for app", "app", depthDependency)
					return reconcile.Result{}, err // restart reconcile
				}

				dApp.Spec.Plan = smallestPlanValue
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

		appInstance.Status.LastStatus = "dependencies_installation_started"
		err = r.Status().Update(ctx, appInstance)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return reconcile.Result{}, err // restart reconcile
		}

		return ctrl.Result{}, nil // stop reconcile
	}

	if lastStatus == "dependencies_installation_started" {
		logger.Info("Entering install stage")
		r.Recorder.Event(appInstance, "Normal", "InstallStarted", "App is entering install stage")

		dependencies, err := utils.GetAppDependencies(appInstance.Spec.Name)
		if err != nil {
			logger.Error(err, "Failed to get all dependencies for app", "app", appInstance.Spec.Name)
			return reconcile.Result{}, err // restart reconcile
		}

		if len(dependencies) > 0 {
			for _, dependency := range dependencies {
				dApp := &appv1alpha1.App{}
				err := r.Get(context.Background(), types.NamespacedName{Namespace: "kubemart-system", Name: dependency}, dApp)
				if err != nil {
					logger.Error(err, "Failed to get dependency", "name", dependency)
					return reconcile.Result{}, err // restart reconcile
				}

				if dApp.Status.LastStatus != "installation_finished" {
					logger.Info("Waiting for dependency installation to complete", "app", appInstance.Spec.Name, "dependency", dependency)
					return ctrl.Result{RequeueAfter: pollIntervalSeconds * time.Second}, nil
				}
			}
		}

		job := newJobPod(appInstance, false)
		// Set App instance as the owner of the Job
		if err := controllerutil.SetControllerReference(appInstance, job, r.Scheme); err != nil {
			// Restart the Reconcile
			return reconcile.Result{}, err
		}

		// Create the Job
		err = r.Create(context.Background(), job)
		if err != nil {
			logger.Error(err, "Failed to create job", "job name", job.Name)
			// Restart the Reconcile
			return reconcile.Result{}, err
		}
		logger.Info("New Job was launched successfully", "job name", job.Name)

		// Refresh the App instance first so we can update it
		err = r.Get(ctx, req.NamespacedName, appInstance)
		if err != nil {
			logger.Error(err, "Failed to refresh App instance")
			// Restart the Reconcile
			return reconcile.Result{}, err
		}

		// Prepare status update
		timeNow := metav1.Now()
		jobStatus := "installation_started"
		jobsExecuted := appInstance.Status.JobsExecuted
		if len(jobsExecuted) == 0 {
			jobsExecuted = make(map[string]appv1alpha1.JobInfo)
		}

		jobsExecuted[job.Name] = appv1alpha1.JobInfo{
			JobStatus: jobStatus,
			StartedAt: &timeNow,
		}

		appInstance.Status.LastStatus = jobStatus
		appInstance.Status.LastJobExecuted = job.Name
		appInstance.Status.JobsExecuted = jobsExecuted

		// Perform status update
		err = r.Status().Update(ctx, appInstance)
		if err != nil {
			logger.Error(err, "Failed to update status")
			return reconcile.Result{}, err // restart the reconcile
		}

		// Create JobWatcher for Job
		// Set the JobWatcher name with this template: app-jw-randomID
		// Example:
		// * wordpress-jw-hkrb
		jobWatcherNameTemplate := fmt.Sprintf("%s-jw-", appInstance.ObjectMeta.Name)
		jobwatcher := &appv1alpha1.JobWatcher{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: jobWatcherNameTemplate,
				Namespace:    "kubemart-system",
			},
			Spec: appv1alpha1.JobWatcherSpec{
				Namespace:  "kubemart-system",
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

		return ctrl.Result{}, nil // stop reconcile
	}

	if lastStatus == "installation_finished" {
		r.Recorder.Event(appInstance, "Normal", "InstallFinished", "App is successfully installed")
		appName := appInstance.ObjectMeta.Name

		if appInstance.Status.InstalledVersion == "" {
			version, err := utils.GetAppVersion(appName)
			if err != nil {
				logger.Error(err, "Unable to get app's version")
				return reconcile.Result{}, nil // stop reconcile
			}

			appInstance.Status.InstalledVersion = version
			logger.Info("Updating installed_version status field", "app", appName)
			err = r.Status().Update(ctx, appInstance)
			if err != nil {
				logger.Error(err, "Failed to update status")
				return reconcile.Result{}, err // restart reconcile
			}
			logger.Info("Successfully updated installed_version status field", "app", appName)
		}

		_, updateWatcherExists := updateWatcher[appName]
		if !updateWatcherExists {
			go r.WatchForNewUpdate(appInstance)
			updateWatcher[appName] = true
		}

		return ctrl.Result{}, nil // stop reconcile
	}

	return ctrl.Result{}, nil // stop reconcile
}

// IsConfigurationExists returns 'true' if the App's configurations contain
// the lookup key (configKey)
func (r *AppReconciler) IsConfigurationExists(app *appv1alpha1.App, configKey string) bool {
	for _, config := range app.Status.Configurations {
		if configKey == config.Key {
			return true
		}
	}

	return false
}

// WatchForNewUpdate will run in Go routine. Each installed App will have an update
// watcher running in Go routine to periodically check if the installed app is latest or not
func (r *AppReconciler) WatchForNewUpdate(appInstance *appv1alpha1.App) {
	logger := r.Log
	appName := appInstance.ObjectMeta.Name
	logger.Info("Adding an update watcher for app", "app", appName)

	for {
		// It's important to have this sleep, because this function will run forever
		// in background as Go routine. Also, this must placed at top/first in this loop.
		// If this sleep is placed at the very bottom, the `continue` syntax below
		// will ignore it and causing massive noises when some errors happen.
		time.Sleep(updateWatcherSleepMinutes * time.Minute)

		// When the App has been deleted, we no longer need this Go routine
		_, exists := updateWatcher[appName]
		if !exists {
			break // terminate Go routine
		}

		logger.Info("Checking for new update", "app", appName)
		versionFromManifest, err := utils.GetAppVersion(appName)
		if err != nil {
			logger.Error(err, "Unable to get app's version")
			continue // try again in next cycle
		}

		// get a fresh app instance
		app := &v1alpha1.App{}
		err = r.Get(context.Background(), types.NamespacedName{
			Namespace: appInstance.ObjectMeta.Namespace,
			Name:      appInstance.ObjectMeta.Name,
		}, app)
		if err != nil {
			if errors.IsNotFound(err) {
				break // terminate Go routine
			}

			logger.Error(err, "Unable to refresh app instance")
			continue // try again in next cycle
		}

		installedVersion := app.Status.InstalledVersion
		if installedVersion != "" && installedVersion != versionFromManifest {
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
	}
}

// DeleteUpdateWatcher will deregister the update watcher by deleting the watcher
// from the watchers list
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

// RemoveFinalizerFromApp - TODO: write description
func (r *AppReconciler) RemoveFinalizerFromApp(appName string) error {
	logger := r.Log.WithValues("app", appName)
	apps := &v1alpha1.AppList{}
	err := r.List(context.Background(), apps)
	if err != nil {
		return err
	}

	found := false
	app := &v1alpha1.App{}
	for _, a := range apps.Items {
		if a.ObjectMeta.Name == appName {
			app = &a
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("app %s not found", appName)
	}

	logger.Info("Removing finalizer")
	app.ObjectMeta.Finalizers = utils.RemoveString(app.ObjectMeta.Finalizers, finalizerName)
	err = r.Update(context.Background(), app)
	if err != nil {
		return err
	}

	logger.Info("Removing terminating app from map")
	delete(terminatingApps, appName)

	return nil
}

// ProcessAppNamespaceDeletion will run after user deletes the App and before the App's
// finalizer get removed. It will delete the app's namespace - which ideally will delete
// all the app's Kubernetes resources.
func (r *AppReconciler) ProcessAppNamespaceDeletion(app *appv1alpha1.App) error {
	appName := app.ObjectMeta.Name
	logger := r.Log.WithValues("app", appName)

	logger.Info("Processing app's namespace deletion...")
	namespace, err := utils.GetNamespaceFromAppManifest(appName)
	if err != nil {
		logger.Error(err, "Unable to determine app namespace from app's manifest.yaml file")
		return r.RemoveFinalizerFromApp(appName)
	}

	if namespace == "" {
		logger.Info("Unable to delete namespace because this app does not have namespace declared in its manifest.yaml file")
		return r.RemoveFinalizerFromApp(appName)
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
	for {
		nsExists, err := r.IsNamespaceExist(namespace)
		if err != nil {
			logger.Error(err, "Unable to check namespace")
			return err
		}

		if !nsExists {
			break
		}
	}

	return r.RemoveFinalizerFromApp(appName)
}

// GetApps will return all created Apps in form of AppList object
func (r *AppReconciler) GetApps() (*appv1alpha1.AppList, error) {
	apps := &appv1alpha1.AppList{}
	err := r.List(context.Background(), apps, &client.ListOptions{})
	if err != nil {
		return apps, err
	}

	return apps, nil
}

// IsAppAlreadyCreated will return 'true' if the lookup app (appName)
// is already created. Default to 'false'.
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

// GetInstalledAppNamesMap will return all Apps that have
// 'installation_finished' status in form of map
func (r *AppReconciler) GetInstalledAppNamesMap() (map[string]bool, error) {
	installedAppsMap := make(map[string]bool)

	apps, err := r.GetApps()
	if err != nil {
		return installedAppsMap, err
	}

	for _, app := range apps.Items {
		if app.Status.LastStatus == "installation_finished" {
			installedAppsMap[app.Spec.Name] = true
		}
	}

	return installedAppsMap, nil
}

// GetKubemartConfigMap will fetch "kubemart-config" ConfigMap and returns KubemartConfigMap
func (r *AppReconciler) GetKubemartConfigMap() (*KubemartConfigMap, error) {
	bcm := &KubemartConfigMap{}
	configMap := &v1.ConfigMap{}
	err := r.Client.Get(context.Background(), types.NamespacedName{
		Name:      "kubemart-config",
		Namespace: "kubemart-system",
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

// IsNamespaceExist will return 'true' if the lookup namespace is found.
// Default to 'false'.
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

// GetNamespace will search a namespace by name (string) and return Namespace object
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

// DeleteNamespace will delete namespace by name (string)
func (r *AppReconciler) DeleteNamespace(namespace string) error {
	ns, _ := r.GetNamespace(namespace)
	err := r.Client.Delete(context.Background(), ns, &client.DeleteOptions{})
	return err
}

// newJobPod is the pod definition of the kubemart-daemon which contains
// helm, kubectl, curl, git & marketplace code. This daemon pod is the app installer pod.
func newJobPod(cr *appv1alpha1.App, isUninstall bool) *batchv1.Job {
	daemonScriptFile := "./install.sh"
	if isUninstall {
		daemonScriptFile = "./uninstall.sh"
	}

	// auto delete the Job (and its Pod) 24 hours after it finishes
	// https://kubernetes.io/docs/concepts/workloads/controllers/job/#ttl-mechanism-for-finished-jobs
	secondsInADay := int32(86400)

	// Set job name with the following format: app-job-randomID
	// Example:
	// * wordpress-job-jzxbw
	jobNameTemplate := fmt.Sprintf("%s-job-", cr.Name)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: jobNameTemplate,
			Namespace:    cr.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &secondsInADay,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "kubemart-daemon-svc-acc",
					RestartPolicy:      "OnFailure",
					Containers: []corev1.Container{
						{
							Name:            "kubemart-daemon",
							Image:           "kubemart/kubemart-daemon:v1alpha1",
							ImagePullPolicy: corev1.PullAlways,
							Command: []string{
								"/bin/sh", "-c",
							},
							Args: []string{
								// Note:
								// The `--namespace` is the namespace where the App custom resource is running.
								// Not where the actual workload i.e. wordpress is running.
								fmt.Sprintf("./main --app-name %s --namespace kubemart-system && cd scripts && %s", cr.Spec.Name, daemonScriptFile),
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
