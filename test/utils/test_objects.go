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

package utils

import (
	"fmt"

	farosv1alpha1 "github.com/pusher/faros/pkg/apis/faros/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// SetGitTrackObjectInterfaceSpec updates the spec of a GitTrackObjectInterface
// to match the given Object
func SetGitTrackObjectInterfaceSpec(gto farosv1alpha1.GitTrackObjectInterface, obj Object) error {
	content, err := runtime.NewTestUnstructuredConverter(apiequality.Semantic).ToUnstructured(obj.DeepCopyObject())
	if err != nil {
		return fmt.Errorf("unable to create unstructured content from object: %v", err)
	}
	u := unstructured.Unstructured{
		Object: content,
	}
	json, err := u.MarshalJSON()
	if err != nil {
		return fmt.Errorf("unable to marshal unstructured json: %v", err)
	}
	if obj.GetObjectKind().GroupVersionKind().Kind == "" {
		return fmt.Errorf("kind not set on object")
	}

	gto.SetSpec(farosv1alpha1.GitTrackObjectSpec{
		Kind: obj.GetObjectKind().GroupVersionKind().Kind,
		Name: obj.GetName(),
		Data: json,
	})
	return nil
}

// ExampleGitTrack is an example GitTrack object for use within test suites
var ExampleGitTrack = &farosv1alpha1.GitTrack{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "faros.pusher.com/v1alpha1",
		Kind:       "GitTrack",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "example",
		Namespace: "default",
	},
	Spec: farosv1alpha1.GitTrackSpec{},
}

// ExampleClusterGitTrack is an example ClusterGitTrack object for use within test suites
var ExampleClusterGitTrack = &farosv1alpha1.ClusterGitTrack{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "faros.pusher.com/v1alpha1",
		Kind:       "ClusterGitTrack",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "example",
	},
	Spec: farosv1alpha1.GitTrackSpec{},
}

// ExampleGitTrackObject is an example GitTrackObject object for use within test suites
var ExampleGitTrackObject = &farosv1alpha1.GitTrackObject{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "faros.pusher.com/v1alpha1",
		Kind:       "GitTrackObject",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "example",
		Namespace: "default",
	},
	Spec: farosv1alpha1.GitTrackObjectSpec{
		Name: "deployment-example",
		Kind: "Deployment",
		Data: []byte(`apiVersion: apps/v1
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
		`),
	},
}

// ExampleClusterGitTrackObject is an example ClusterGitTrackObject object for use within test suites
var ExampleClusterGitTrackObject = &farosv1alpha1.ClusterGitTrackObject{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "faros.pusher.com/v1alpha1",
		Kind:       "ClusterGitTrackObject",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "example",
	},
	Spec: farosv1alpha1.GitTrackObjectSpec{
		Name: "clusterrolebinding-example",
		Kind: "ClusterRoleBinding",
		Data: []byte(`apiVersion: rbac.authorization.k8s.io/v1
		kind: ClusterRoleBinding
		metadata:
		  name: example
		  labels:
		    app: nginx
		roleRef:
		  apiGroup: rbac.authorization.k8s.io
		  kind: ClusterRole
		  name: nginx-ingress-controller
		subjects:
		- kind: ServiceAccount
		  name: nginx-ingress-controller
		  namespace: example
		`),
	},
}

var appNginx = map[string]string{
	"app": "nginx",
}

// ExampleDeployment is an example Deployment object for use within test suites
var ExampleDeployment = &appsv1.Deployment{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "example",
		Namespace: "default",
		Labels:    appNginx,
	},
	Spec: appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: appNginx,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: appNginx,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx",
					},
				},
			},
		},
	},
}

// ExampleClusterRoleBinding is an example ClusterRoleBinding object for use within test suites
var ExampleClusterRoleBinding = &rbacv1.ClusterRoleBinding{
	TypeMeta: metav1.TypeMeta{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRoleBinding",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:   "example",
		Labels: appNginx,
	},
	RoleRef: rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     "nginx-ingress-controller",
	},
	Subjects: []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      "nginx-ingress-controller",
			Namespace: "example",
		},
	},
}
