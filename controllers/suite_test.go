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
	"os"
	"path/filepath"
	"testing"

	appv1alpha1 "github.com/kubemart/kubemart-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	// logf "sigs.k8s.io/controller-runtime/pkg/log"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var directClient client.Client
var testEnv *envtest.Environment
var systemNamespace = "kubemart-system"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	// logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	// fmt.Println("Bootstrapping test environment")
	os.Setenv("USE_EXISTING_CLUSTER", "true")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = appv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&AppReconciler{
		Client:   k8sManager.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("App"),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("kubemart-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&JobWatcherReconciler{
		Client:   k8sManager.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("JobWatcher"),
		Scheme:   k8sManager.GetScheme(),
		Recorder: k8sManager.GetEventRecorderFor("kubemart-controller"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	// https://github.com/kubernetes-sigs/controller-runtime/issues/343#issuecomment-469435686
	directClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())

	// Create Kubemart Namespace
	err = createKubemartNamespaceIfNotExist(directClient)
	Expect(err).ToNot(HaveOccurred())

	// Create Kubemart ConfigMap
	err = createKubemartConfigMapIfNotExist(directClient)
	Expect(err).ToNot(HaveOccurred())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	// fmt.Printf("\nTearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

// --------------------------------------------------------

func createKubemartNamespaceIfNotExist(dc client.Client) error {
	namespace := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: systemNamespace,
		},
	}

	ns := &v1.Namespace{}
	err := dc.Get(context.Background(), types.NamespacedName{
		Name: systemNamespace,
	}, ns)
	if err == nil {
		return nil
	}

	err = dc.Create(context.Background(), namespace, &client.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func createKubemartConfigMapIfNotExist(dc client.Client) error {
	configMapName := "kubemart-config"
	configMapData := make(map[string]string)
	configMapData["email"] = "test@example.com"
	configMapData["domain"] = "example.com"
	configMapData["cluster_name"] = "example-cluster"
	configMapData["master_ip"] = "123.456.789.101"

	configMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: systemNamespace,
		},
		Data: configMapData,
	}

	cm := &v1.ConfigMap{}
	err := dc.Get(context.Background(), types.NamespacedName{
		Namespace: systemNamespace,
		Name:      configMapName,
	}, cm)
	if err == nil {
		return nil
	}

	err = dc.Create(context.Background(), configMap, &client.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}
