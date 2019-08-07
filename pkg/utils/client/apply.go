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

// Package client has been copied from kubectl code almost verbatim
// Some methods have been moved from k/k into the utils.go file to remove the
// dependency on K/K
// https://github.com/kubernetes/kubernetes/blob/v1.13.1/pkg/kubectl/cmd/apply/apply.go
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/jonboulle/clockwork"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	rlogr "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

// Options are creation options for a Applier
type Options struct {
	// Scheme, if provided, will be used to map go structs to GroupVersionKinds
	Scheme *runtime.Scheme

	// Mapper, if provided, will be used to map GroupVersionKinds to Resources
	Mapper meta.RESTMapper
}

// Client defines the interface for the Applier
type Client interface {
	Apply(context.Context, *ApplyOptions, runtime.Object) error
}

// Make sure Applier implements Client
var _ Client = &Applier{}

// Applier is a client that can perform a three-way-merge
// Based on the `kubectl apply` command
type Applier struct {
	mapper meta.RESTMapper
	scheme *runtime.Scheme

	client        client.Client
	dynamicClient dynamic.Interface
	config        *rest.Config
	codecs        serializer.CodecFactory
	log           logr.Logger
}

// NewApplier constucts a new Applier client
func NewApplier(config *rest.Config, options Options) (*Applier, error) {
	if config == nil {
		return nil, fmt.Errorf("must provide non-nil rest.Config to client.New")
	}

	// Init a scheme if none provided
	if options.Scheme == nil {
		options.Scheme = scheme.Scheme
	}

	// Init a Mapper if none provided
	if options.Mapper == nil {
		var err error

		drm, err := apiutil.NewDiscoveryRESTMapper(config)
		if err != nil {
			return nil, err
		}
		lrm := meta.NewLazyRESTMapperLoader(func() (meta.RESTMapper, error) {
			return apiutil.NewDiscoveryRESTMapper(config)
		})

		options.Mapper = meta.FirstHitRESTMapper{MultiRESTMapper: meta.MultiRESTMapper{drm, lrm}}
	}

	cachingClient, err := client.New(config, client.Options{Scheme: options.Scheme, Mapper: options.Mapper})
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	a := &Applier{
		mapper:        options.Mapper,
		scheme:        options.Scheme,
		codecs:        serializer.NewCodecFactory(options.Scheme),
		client:        cachingClient,
		dynamicClient: dynamicClient,
		config:        config,
		log:           rlogr.Log.WithName("applier"),
	}

	return a, nil
}

// ApplyOptions defines the possible options for the Apply command
type ApplyOptions struct {
	Overwrite           *bool // Automatically resolve conflicts between the modified and live configuration by using values from the modified configuration
	ForceDeletion       *bool
	CascadeDeletion     *bool
	DeletionTimeout     *time.Duration
	DeletionGracePeriod *int
	ServerDryRun        *bool
}

// Complete defaults valus within the ApplyOptions struct
func (a *ApplyOptions) Complete() {
	// setup option defaults
	overwrite := true
	forceDeletion := false
	cascadeDeletion := true
	deletionTimeout := time.Duration(30 * time.Second)
	deletionGracePeriod := -1
	serverDryRun := false

	if a.Overwrite == nil {
		a.Overwrite = &overwrite
	}
	if a.ForceDeletion == nil {
		a.ForceDeletion = &forceDeletion
	}
	if a.CascadeDeletion == nil {
		a.CascadeDeletion = &cascadeDeletion
	}
	if a.DeletionTimeout == nil {
		a.DeletionTimeout = &deletionTimeout
	}
	if a.DeletionGracePeriod == nil {
		a.DeletionGracePeriod = &deletionGracePeriod
	}
	if a.ServerDryRun == nil {
		a.ServerDryRun = &serverDryRun
	}
}

// Apply performs a strategic three way merge update to the resource if it exists,
// else it creates the resource
func (a *Applier) Apply(ctx context.Context, opts *ApplyOptions, modified runtime.Object) error {
	// Default option values
	opts.Complete()

	current := newUnstructuredFor(modified)

	objectKey, err := getNamespacedName(modified)
	if err != nil {
		return fmt.Errorf("unable to determine NamespacedName: %v", err)
	}

	// Check if the resource already exists
	err = a.client.Get(context.TODO(), objectKey, current)
	if err != nil && errors.IsNotFound(err) {
		// Object is not found, create it
		return a.create(ctx, opts, modified)
	} else if err != nil {
		return fmt.Errorf("unable to get current resource: %v", err)
	}
	// Update the object
	err = a.update(ctx, opts, current, modified)
	if err != nil {
		return fmt.Errorf("error applying update: %v", err)
	}

	return nil
}

func (a *Applier) create(ctx context.Context, opts *ApplyOptions, obj runtime.Object) error {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("unable to read metadata from object: %v", err)
	}
	log := a.log.WithValues(
		"kind", obj.GetObjectKind().GroupVersionKind().String(),
		"name", metadata.GetName(),
		"namespace", metadata.GetNamespace(),
	)
	log.V(2).Info("creating resource", "dry-run", *opts.ServerDryRun)

	err = createApplyAnnotation(obj, unstructured.UnstructuredJSONScheme)
	if err != nil {
		return fmt.Errorf("unable to apply LastAppliedAnnotation to object: %v", err)
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	restClient, err := a.restClientFor(gvk.GroupVersion())
	if err != nil {
		return fmt.Errorf("unable to construct REST client for GroupVersion %s: %v", gvk.GroupVersion().String(), err)
	}

	mapping, err := a.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("unable to get REST mapping for GroupVersionKind %s: %v", gvk.String(), err)
	}

	createOptions := &metav1.CreateOptions{}
	if *opts.ServerDryRun {
		createOptions.DryRun = []string{metav1.DryRunAll}
	}

	err = restClient.Post().
		NamespaceIfScoped(metadata.GetNamespace(), isNamespaced(mapping)).
		Resource(mapping.Resource.Resource).
		Body(obj).
		VersionedParams(createOptions, metav1.ParameterCodec).
		Context(ctx).
		Do().
		Into(obj)
	if err != nil {
		return fmt.Errorf("error creating object: %v", err)
	}
	return nil
}

func (a *Applier) update(ctx context.Context, opts *ApplyOptions, current, modified runtime.Object) error {
	metadata, err := meta.Accessor(modified)
	if err != nil {
		return fmt.Errorf("unable to get object metadata: %v", err)
	}

	log := a.log.WithValues(
		"kind", modified.GetObjectKind().GroupVersionKind().String(),
		"name", metadata.GetName(),
		"namespace", metadata.GetNamespace(),
	)
	log.V(2).Info("updating resource", "dry-run", *opts.ServerDryRun)

	modifiedJSON, err := getModifiedConfiguration(modified, true, unstructured.UnstructuredJSONScheme)
	if err != nil {
		return fmt.Errorf("unable to get modified configuration: %v", err)
	}

	patcher, err := a.newPatcher(opts, modified)
	if err != nil {
		return fmt.Errorf("unable to construct patcher: %v", err)
	}
	source := metadata.GetSelfLink() // This is optional and would normally be the file path
	_, patchedObj, err := patcher.Patch(current, modifiedJSON, source, metadata.GetNamespace(), metadata.GetName(), nil)
	if err != nil {
		return fmt.Errorf("unable to patch object: %v", err)
	}

	// Copy the patchedObj into the modified runtime.Object
	err = a.copyInto(patchedObj, modified)
	if err != nil {
		return fmt.Errorf("error copying response: %v", err)
	}
	return nil
}

func (a *Applier) newPatcher(opts *ApplyOptions, obj runtime.Object) (*Patcher, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	mapping, err := a.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("couldn't construct rest mapping from GVK %s: %v", gvk.String(), err)
	}

	restClient, err := a.restClientFor(gvk.GroupVersion())
	if err != nil {
		return nil, fmt.Errorf("unable to get REST Client: %v", err)
	}

	helper := resource.NewHelper(restClient, mapping)
	p := &Patcher{
		Mapping:       mapping,
		Helper:        helper,
		DynamicClient: a.dynamicClient,
		Overwrite:     *opts.Overwrite,
		BackOff:       clockwork.NewRealClock(),
		Force:         *opts.ForceDeletion,
		Cascade:       *opts.CascadeDeletion,
		Timeout:       *opts.DeletionTimeout,
		GracePeriod:   *opts.DeletionGracePeriod,
		ServerDryRun:  *opts.ServerDryRun,
		OpenapiSchema: nil, // Not supporting OpenapiSchema patching
		Retries:       maxPatchRetry,
	}
	return p, nil
}

func (a *Applier) configFor(gv schema.GroupVersion) (*rest.Config, error) {
	config := rest.CopyConfig(a.config)
	err := rest.SetKubernetesDefaults(config)
	if err != nil {
		return nil, fmt.Errorf("error defaulting config: %v", err)
	}

	config.GroupVersion = &gv

	// Set correct APIPath for core API group
	if gv.String() == "v1" {
		config.APIPath = "api/"
	} else {
		config.APIPath = "apis/"
	}

	contentConfig := resource.UnstructuredPlusDefaultContentConfig()
	config.NegotiatedSerializer = contentConfig.NegotiatedSerializer
	return config, nil
}

func newUnstructuredFor(obj runtime.Object) *unstructured.Unstructured {
	gvk := obj.GetObjectKind().GroupVersionKind()
	apiVersion, kind := gvk.ToAPIVersionAndKind()

	u := &unstructured.Unstructured{}
	u.SetKind(kind)
	u.SetAPIVersion(apiVersion)

	return u
}

func (a *Applier) restClientFor(gv schema.GroupVersion) (rest.Interface, error) {
	restConfig, err := a.configFor(gv)
	if err != nil {
		return nil, fmt.Errorf("failed to construct config for Group Version %+v: %v", gv, err)
	}
	restClient, err := rest.UnversionedRESTClientFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise rest client: %v", err)
	}
	return restClient, nil
}

func (a *Applier) copyInto(in, out runtime.Object) error {
	data, err := json.Marshal(in)
	if err != nil {
		return err
	}
	gvk := in.GetObjectKind().GroupVersionKind()
	_, _, err = a.codecs.UniversalDecoder().Decode(data, &gvk, out)
	if err != nil {
		return err
	}
	return nil
}

func isNamespaced(mapping *meta.RESTMapping) bool {
	if mapping.Scope.Name() == meta.RESTScopeNameRoot {
		return false
	}
	return true
}
