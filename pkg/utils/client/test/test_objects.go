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

package test

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
