/*
Copyright 2018 Pusher Ltd.

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

package gittrackobject

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	"github.com/pusher/faros/pkg/utils"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var exampleDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
  namespace: default
  labels:
    app: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
     labels:
       app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
`

var exampleDeployment2 = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
  namespace: default
  labels:
    app: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
`

var invalidExample = `apiVersion: apps/v1
kind: Deployment
metadata:
name: example
namespace: default
labels:
app: nginx
spec:
selector:
matchLabels:
app: nginx
template:
metadata:
labels:
app: nginx
spec:
containers:
- name: nginx
image: nginx:latest
`

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "example", Namespace: "default"}}
var depKey = types.NamespacedName{Name: "example", Namespace: "default"}
var mgr manager.Manager
var instance *farosv1alpha1.GitTrackObject
var requests chan reconcile.Request
var stop chan struct{}
var stopInformers chan struct{}
var exampleDeploymentJSON []byte
var exampleDeploymentJSON2 []byte

func init() {
	example, _ := utils.YAMLToUnstructured([]byte(exampleDeployment))
	exampleDeploymentJSON, _ = example.MarshalJSON()
	example2, _ := utils.YAMLToUnstructured([]byte(exampleDeployment2))
	exampleDeploymentJSON2, _ = example2.MarshalJSON()
}

const timeout = time.Second * 5

var _ = Describe("GitTrackObject Suite", func() {
	BeforeEach(func() {
		// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
		// channel when it is finished.
		var err error
		mgr, err = manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		var recFn reconcile.Reconciler
		testReconciler := newReconciler(mgr)
		recFn, requests = SetupTestReconcile(testReconciler)
		Expect(add(mgr, recFn)).NotTo(HaveOccurred())
		stopInformers = testReconciler.(Reconciler).StopChan()
		stop = StartTestManager(mgr)
	})

	AfterEach(func() {
		close(stop)
		close(stopInformers)
	})

	Describe("When a GitTrackObject resource is created (with YAML)", func() {
		BeforeEach(func() { CreateInstance([]byte(exampleDeployment)) })
		AfterEach(DeleteInstance)

		It("should create it's child resource", ShouldCreateChild)

		It("should add an owner reference to the child", ShouldAddOwnerReference)

		It("should update the GTO status", func() {
			ShouldUpdateConditionStatus(v1.ConditionTrue)
		})

		It("should update the resource when the GTO is updated", func() {
			ShouldUpdateChildOnGTOUpdate([]byte(exampleDeployment2))
		})

		It("should recreate the child if it is deleted", ShouldRecreateChildIfDeleted)

		It("should reset the child if it is modified", ShouldResetChildIfModified)
	})

	Describe("When a GitTrackObject resource is created (with JSON)", func() {
		BeforeEach(func() { CreateInstance(exampleDeploymentJSON) })
		AfterEach(DeleteInstance)

		It("should create it's child resource", ShouldCreateChild)

		It("should add an owner reference to the child", ShouldAddOwnerReference)

		It("should update the GTO status", func() {
			ShouldUpdateConditionStatus(v1.ConditionTrue)
		})

		It("should update the resource when the GTO is updated", func() {
			ShouldUpdateChildOnGTOUpdate(exampleDeploymentJSON2)
		})

		It("should recreate the child if it is deleted", ShouldRecreateChildIfDeleted)

		It("should reset the child if it is modified", ShouldResetChildIfModified)
	})

	Describe("When a GitTrackObject resource is created (with invalid data)", func() {
		BeforeEach(func() { CreateInstance([]byte(invalidExample)) })
		AfterEach(DeleteInstance)

		It("should set the status to failed", func() {
			ShouldUpdateConditionStatus(v1.ConditionFalse)
		})

	})
})

var (
	CreateInstance = func(data []byte) {
		instance = &farosv1alpha1.GitTrackObject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example",
				Namespace: "default",
			},
			Spec: farosv1alpha1.GitTrackObjectSpec{
				Name: "deployment-example",
				Kind: "Deployment",
				Data: data,
			},
		}
		// Create the GitTrackObject object and expect the Reconcile to occur
		err := c.Create(context.TODO(), instance)
		Expect(err).NotTo(HaveOccurred())
		Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
	}

	DeleteInstance = func() {
		err := c.Delete(context.TODO(), instance)
		Expect(err).NotTo(HaveOccurred())
	}

	ShouldCreateChild = func() {
		deploy := &appsv1.Deployment{}
		Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
			Should(Succeed())

		// GC not enabled so manually delete the object
		Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
	}

	ShouldAddOwnerReference = func() {
		deploy := &appsv1.Deployment{}
		Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
			Should(Succeed())

		Expect(len(deploy.OwnerReferences)).To(Equal(1))
		oRef := deploy.OwnerReferences[0]
		Expect(oRef.APIVersion).To(Equal("faros.pusher.com/v1alpha1"))
		Expect(oRef.Kind).To(Equal("GitTrackObject"))
		Expect(oRef.Name).To(Equal(instance.Name))

		// GC not enabled so manually delete the object
		Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
	}

	ShouldUpdateConditionStatus = func(expected v1.ConditionStatus) {
		if expected == v1.ConditionTrue {
			deploy := &appsv1.Deployment{}
			Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
				Should(Succeed())

			// GC not enabled so manually delete the object
			defer Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
		}

		err := c.Get(context.TODO(), depKey, instance)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(instance.Status.Conditions)).To(Equal(1))
		condition := instance.Status.Conditions[0]
		Expect(condition.Type).To(Equal(farosv1alpha1.ObjectInSyncType))
		Expect(condition.Status).To(Equal(expected))

	}

	ShouldUpdateChildOnGTOUpdate = func(data []byte) {
		deploy := &appsv1.Deployment{}
		Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
			Should(Succeed())

		// Fetch the instance and update it
		Eventually(func() error { return c.Get(context.TODO(), depKey, instance) }, timeout).
			Should(Succeed())
		instance.Spec.Data = data
		err := c.Update(context.TODO(), instance)
		Expect(err).ShouldNot(HaveOccurred())

		// Wait for a reconcile to happen
		Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

		Eventually(func() error {
			err := c.Get(context.TODO(), depKey, deploy)
			if err != nil {
				return err
			}
			if len(deploy.Spec.Template.Spec.Containers) != 1 &&
				deploy.Spec.Template.Spec.Containers[0].Image != "nginx:latest" {
				return errors.New("Image not updated")
			}
			return nil
		}, timeout).Should(Succeed())
		// GC not enabled so manually delete the object
		Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
	}

	ShouldRecreateChildIfDeleted = func() {
		deploy := &appsv1.Deployment{}
		Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
			Should(Succeed())

		// Delete the instance and expect it to be recreated
		Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
		Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
		Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
			Should(Succeed())

		// GC not enabled so manually delete the object
		Eventually(func() error { return c.Delete(context.TODO(), deploy) }, timeout).
			Should(Succeed())
	}

	ShouldResetChildIfModified = func() {
		deploy := &appsv1.Deployment{}
		Eventually(func() error { return c.Get(context.TODO(), depKey, deploy) }, timeout).
			Should(Succeed())

		// Update the spec and expect it to be reset
		deploy.Spec.Template.Spec.Containers[0].Image = "nginx:latest"
		Expect(c.Update(context.TODO(), deploy)).To(Succeed())
		Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
		Eventually(func() error {
			err := c.Get(context.TODO(), depKey, deploy)
			if err != nil {
				return err
			}
			if len(deploy.Spec.Template.Spec.Containers) != 1 &&
				deploy.Spec.Template.Spec.Containers[0].Image != "nginx" {
				return errors.New("Image not updated")
			}
			return nil
		}, timeout).Should(Succeed())

		// GC not enabled so manually delete the object
		Expect(c.Delete(context.TODO(), deploy)).To(Succeed())
	}
)
