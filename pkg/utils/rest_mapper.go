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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// NewRestMapper creates a restMapper from the discovery client
func NewRestMapper(config *rest.Config) (meta.RESTMapper, error) {
	client, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create dynamic client: %v", err)
	}

	apiGroupResources, err := restmapper.GetAPIGroupResources(client)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch API Group Resources: %v", err)
	}

	return restmapper.NewDiscoveryRESTMapper(apiGroupResources), nil
}

// GetAPIResource uses a rest mapper to get the GroupVersionResource and
// determine whether an object is namespaced or not
func GetAPIResource(restMapper meta.RESTMapper, gvk schema.GroupVersionKind) (schema.GroupVersionResource, bool, error) {
	fqKind := schema.FromAPIVersionAndKind(gvk.ToAPIVersionAndKind())
	mapping, err := restMapper.RESTMapping(fqKind.GroupKind(), fqKind.Version)
	if err != nil {
		return schema.GroupVersionResource{}, false, fmt.Errorf("unable to get REST mapping: %v", err)
	}
	return mapping.Resource, mapping.Scope == meta.RESTScopeNamespace, nil
}
