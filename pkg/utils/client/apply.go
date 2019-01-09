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
	"fmt"
	"time"

	"github.com/jonboulle/clockwork"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
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
		options.Mapper, err = apiutil.NewDiscoveryRESTMapper(config)
		if err != nil {
			return nil, err
		}
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
}

// Complete defaults valus within the ApplyOptions struct
func (a *ApplyOptions) Complete() {
	// setup option defaults
	overwrite := true
	forceDeletion := false
	cascadeDeletion := true
	deletionTimeout := time.Duration(0)
	deletionGracePeriod := -1

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
		return a.create(ctx, modified.DeepCopyObject())
	} else if err != nil {
		return fmt.Errorf("unable to get current resource: %v", err)
	}
	// Update the object
	return a.update(ctx, opts, current, modified)
}

func (a *Applier) create(ctx context.Context, obj runtime.Object) error {
	err := createApplyAnnotation(obj, unstructured.UnstructuredJSONScheme)
	if err != nil {
		return fmt.Errorf("unable to apply LastAppliedAnnotation to object: %v", err)
	}

	err = a.client.Create(ctx, obj)
	if err != nil {
		return fmt.Errorf("error creating object: %v", err)
	}
	return nil
}

func (a *Applier) update(ctx context.Context, opts *ApplyOptions, current, modified runtime.Object) error {
	modifiedJSON, err := getModifiedConfiguration(modified, true, unstructured.UnstructuredJSONScheme)
	if err != nil {
		return fmt.Errorf("unable to get modified configuration: %v", err)
	}

	metadata, err := meta.Accessor(modified)
	if err != nil {
		return fmt.Errorf("unable to get object metadata: %v", err)
	}

	patcher, err := a.newPatcher(opts, modified)
	if err != nil {
		return fmt.Errorf("unable to construct patcher: %v", err)
	}
	source := metadata.GetSelfLink() // This is optional and would normally be the file path
	_, _, err = patcher.Patch(current, modifiedJSON, source, metadata.GetNamespace(), metadata.GetName(), nil)
	if err != nil {
		return fmt.Errorf("unable to patch object: %v", err)
	}

	return nil
}

func (a *Applier) newPatcher(opts *ApplyOptions, obj runtime.Object) (*Patcher, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	mapping, err := a.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("couldn't construct rest mapping from GVK %s: %v", gvk.String(), err)
	}

	restConfig, err := a.configFor(gvk.GroupVersion())
	if err != nil {
		return nil, fmt.Errorf("error constructing config for Group Version %+v: %v", gvk.GroupVersion(), err)
	}
	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise rest client: %v", err)
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
		ServerDryRun:  false, // TODO(JoelSpeed): Implement ServerDryRun in Apply
		OpenapiSchema: nil,   // Not supporting OpenapiSchema patching
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

	codec := runtime.NoopEncoder{Decoder: scheme.Codecs.UniversalDecoder(gv)}
	config.NegotiatedSerializer = serializer.NegotiatedSerializerWrapper(runtime.SerializerInfo{Serializer: codec})
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
