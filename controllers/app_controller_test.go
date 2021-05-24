package controllers

import (
	"context"
	"time"

	v1alpha1 "github.com/kubemart/kubemart-operator/api/v1alpha1"
	"github.com/kubemart/kubemart-operator/pkg/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout  = time.Minute * 5
	interval = time.Second * 1
)

var _ = Describe("App controller\n", func() {
	Context("When creating a simple (no dependency) App", func() {
		ctx := context.Background()
		appName := "rabbitmq"

		It("Should create the App CR", func() {
			app := &v1alpha1.App{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kubemart.civo.com/v1alpha1",
					Kind:       "App",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: systemNamespace,
				},
				Spec: v1alpha1.AppSpec{
					Name:   appName,
					Action: "install",
				},
			}

			Expect(directClient.Create(ctx, app)).Should(Succeed())
		})

		It("Should have installation_finished status after the installation", func() {
			lookupKey := types.NamespacedName{Name: appName, Namespace: systemNamespace}
			createdApp := &v1alpha1.App{}

			Eventually(func() string {
				err := directClient.Get(ctx, lookupKey, createdApp)
				if err != nil {
					return err.Error()
				}

				return createdApp.Status.LastStatus
			}, timeout, interval).Should(Equal("installation_finished"))
		})

		It("Should have installed version status after the installation", func() {
			lookupKey := types.NamespacedName{Name: appName, Namespace: systemNamespace}
			createdApp := &v1alpha1.App{}

			Eventually(func() string {
				err := directClient.Get(ctx, lookupKey, createdApp)
				if err != nil {
					return err.Error()
				}

				return createdApp.Status.InstalledVersion
			}, timeout, interval).ShouldNot(BeEmpty())
		})

		It("Should have 1 rabbitmq pod created", func() {
			pods := &v1.PodList{}

			Eventually(func() int {
				err := directClient.List(ctx, pods, &client.ListOptions{
					Namespace: "rabbitmq",
					LabelSelector: labels.SelectorFromSet(map[string]string{
						"app": "rabbitmq",
					}),
				})

				if err != nil {
					return -1
				}

				return len(pods.Items)
			}, timeout, interval).Should(Equal(1))
		})
	})

	Context("When creating a complex (with dependencies) App", func() {
		ctx := context.Background()
		appName := "wordpress"
		planSize := "10Gi"
		planSizeInt := utils.ExtractPlanIntFromPlanStr(planSize)
		oneGibInBytes := 1073741824
		planSizeInBytes := planSizeInt * oneGibInBytes

		It("Should create the App CR", func() {
			app := &v1alpha1.App{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kubemart.civo.com/v1alpha1",
					Kind:       "App",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      appName,
					Namespace: systemNamespace,
				},
				Spec: v1alpha1.AppSpec{
					Name:   appName,
					Action: "install",
					Plan:   planSize,
				},
			}

			Expect(directClient.Create(ctx, app)).Should(Succeed())
		})

		It("Should have installation_finished status after the installation", func() {
			lookupKey := types.NamespacedName{Name: appName, Namespace: systemNamespace}
			createdApp := &v1alpha1.App{}

			Eventually(func() string {
				err := directClient.Get(ctx, lookupKey, createdApp)
				if err != nil {
					return err.Error()
				}

				return createdApp.Status.LastStatus
			}, timeout, interval).Should(Equal("installation_finished"))
		})

		It("Should have installed version status after the installation", func() {
			lookupKey := types.NamespacedName{Name: appName, Namespace: systemNamespace}
			createdApp := &v1alpha1.App{}

			Eventually(func() string {
				err := directClient.Get(ctx, lookupKey, createdApp)
				if err != nil {
					return err.Error()
				}

				return createdApp.Status.InstalledVersion
			}, timeout, interval).ShouldNot(BeEmpty())
		})

		It("Should have dependency pod (longhorn) created", func() {
			pods := &v1.PodList{}

			Eventually(func() int {
				err := directClient.List(ctx, pods, &client.ListOptions{
					Namespace: "longhorn-system",
					LabelSelector: labels.SelectorFromSet(map[string]string{
						"app": "longhorn-ui",
					}),
				})

				if err != nil {
					return -1
				}

				return len(pods.Items)
			}, timeout, interval).Should(Equal(1))
		})

		It("Should have dependency pod (mariadb) created", func() {
			pods := &v1.PodList{}

			Eventually(func() int {
				err := directClient.List(ctx, pods, &client.ListOptions{
					Namespace: "mariadb",
					LabelSelector: labels.SelectorFromSet(map[string]string{
						"app": "mariadb",
					}),
				})

				if err != nil {
					return -1
				}

				return len(pods.Items)
			}, timeout, interval).Should(Equal(1))
		})

		It("Should have 1 wordpress pod created", func() {
			pods := &v1.PodList{}

			Eventually(func() int {
				err := directClient.List(ctx, pods, &client.ListOptions{
					// TODO - uncomment this after we add namespace to WP app's manifest.yaml
					// Namespace: "mariadb",
					LabelSelector: labels.SelectorFromSet(map[string]string{
						"app": "wordpress",
					}),
				})

				if err != nil {
					return -1
				}

				return len(pods.Items)
			}, timeout, interval).Should(Equal(1))
		})

		It("Should create 2 PVCs - for mariadb and wordpress", func() {
			pvcs := &v1.PersistentVolumeClaimList{}

			Eventually(func() int {
				err := directClient.List(ctx, pvcs, &client.ListOptions{})

				if err != nil {
					return -1
				}

				return len(pvcs.Items)
			}, timeout, interval).Should(Equal(2))

			pvcNames := []string{}
			pvcSizes := make(map[string]int)
			for _, pvc := range pvcs.Items {
				pvcName := pvc.Name
				pvcSize, _ := pvc.Spec.Resources.Requests.Storage().AsInt64()
				pvcNames = append(pvcNames, pvcName)
				pvcSizes[pvcName] = int(pvcSize)
			}

			Expect(pvcNames).To(ContainElement("mariadb-pv-claim"))
			Expect(pvcNames).To(ContainElement("wordpress-pv-claim"))
			Expect(pvcSizes["wordpress-pv-claim"]).To(Equal(planSizeInBytes))
		})

		It("Should create 2 Services - for mariadb and wordpress", func() {
			svcs := &v1.ServiceList{}

			Eventually(func() bool {
				err := directClient.List(ctx, svcs, &client.ListOptions{})

				if err != nil {
					return false
				}

				return true
			}, timeout, interval).Should(BeTrue())

			svcNames := []string{}
			for _, svc := range svcs.Items {
				svcNames = append(svcNames, svc.Name)
			}

			Expect(svcNames).To(ContainElement("mariadb"))
			Expect(svcNames).To(ContainElement("wordpress"))
		})

		It("Should create 1 Ingress - for wordpress", func() {
			ings := &v1beta1.IngressList{}

			Eventually(func() int {
				err := directClient.List(ctx, ings, &client.ListOptions{})

				if err != nil {
					return -1
				}

				return len(ings.Items)
			}, timeout, interval).Should(Equal(1))

			ingNames := []string{}
			for _, ing := range ings.Items {
				ingNames = append(ingNames, ing.Name)
			}

			Expect(ingNames).To(ContainElement("wordpress"))
		})

		It("Should create configurations for mariadb", func() {
			lookupKey := types.NamespacedName{Name: "mariadb", Namespace: systemNamespace}
			createdApp := &v1alpha1.App{}

			Eventually(func() []v1alpha1.Configuration {
				err := directClient.Get(ctx, lookupKey, createdApp)
				if err != nil {
					return nil
				}

				return createdApp.Status.Configurations
			}, timeout, interval).ShouldNot(BeNil())

			confs := make(map[string]v1alpha1.Configuration)
			for _, c := range createdApp.Status.Configurations {
				confs[c.Key] = c
			}

			Expect(confs["MYSQL_ROOT_PASSWORD"].Value).ShouldNot(BeEmpty())
			Expect(confs["MYSQL_ROOT_PASSWORD"].ValueIsBase64).Should(BeTrue())
		})

		It("Should create configurations for wordpress", func() {
			lookupKey := types.NamespacedName{Name: appName, Namespace: systemNamespace}
			createdApp := &v1alpha1.App{}

			Eventually(func() []v1alpha1.Configuration {
				err := directClient.Get(ctx, lookupKey, createdApp)
				if err != nil {
					return nil
				}

				return createdApp.Status.Configurations
			}, timeout, interval).ShouldNot(BeNil())

			confs := make(map[string]v1alpha1.Configuration)
			for _, c := range createdApp.Status.Configurations {
				confs[c.Key] = c
			}

			Expect(confs["DOMAIN_NAME"].Value).ShouldNot(BeEmpty())
			Expect(confs["DOMAIN_NAME"].ValueIsBase64).Should(BeFalse())
		})

		It("Should have 1 job executed (that installs wordpress)", func() {
			lookupKey := types.NamespacedName{Name: appName, Namespace: systemNamespace}
			createdApp := &v1alpha1.App{}

			Eventually(func() int {
				err := directClient.Get(ctx, lookupKey, createdApp)
				if err != nil {
					return -1
				}

				return len(createdApp.Status.JobsExecuted)
			}, timeout, interval).Should(Equal(1))
		})
	})

})
