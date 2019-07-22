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

// Generate deepcopy for apis
//go:generate go run ../../vendor/k8s.io/code-generator/cmd/deepcopy-gen/main.go -O zz_generated.deepcopy -i ./... -h ../../hack/boilerplate.go.txt

// Generate clientset for apis
//go:generate go run ../../vendor/k8s.io/code-generator/cmd/client-gen/main.go --input-base=github.com/pusher/faros/pkg/apis --input="faros/v1alpha1" --input="faros/v1alpha2" -n clientset -p github.com/pusher/faros/pkg/client -h ../../hack/boilerplate.go.txt

// Generate listers for apis
//go:generate go run ../../vendor/k8s.io/code-generator/cmd/lister-gen/main.go --input-dirs=github.com/pusher/faros/pkg/apis/faros/v1alpha1,github.com/pusher/faros/pkg/apis/faros/v1alpha2 -p github.com/pusher/faros/pkg/client/listers -h ../../hack/boilerplate.go.txt

// Generate informers for apis
//go:generate go run ../../vendor/k8s.io/code-generator/cmd/informer-gen/main.go --input-dirs=github.com/pusher/faros/pkg/apis/faros/v1alpha1,github.com/pusher/faros/pkg/apis/faros/v1alpha2 -p github.com/pusher/faros/pkg/client/informers --listers-package github.com/pusher/faros/pkg/client/listers --versioned-clientset-package github.com/pusher/faros/pkg/client/clientset -h ../../hack/boilerplate.go.txt

// Package apis contains Kubernetes API groups.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
