/*
Copyright 2016 The Kubernetes Authors.

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

package apiserver

import (
	"k8s.io/apimachinery/pkg/apimachinery/announced"
	"k8s.io/apimachinery/pkg/apimachinery/registered"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/munnerz/k8s-api-pager-demo/pkg/apis/pager"
	"github.com/munnerz/k8s-api-pager-demo/pkg/apis/pager/install"
	"github.com/munnerz/k8s-api-pager-demo/pkg/apis/pager/v1alpha1"
	informers "github.com/munnerz/k8s-api-pager-demo/pkg/client/informers/internalversion"
	alertstorage "github.com/munnerz/k8s-api-pager-demo/pkg/registry/pager/alert"
)

var (
	groupFactoryRegistry = make(announced.APIGroupFactoryRegistry)
	registry             = registered.NewOrDie("")
	Scheme               = runtime.NewScheme()
	Codecs               = serializer.NewCodecFactory(Scheme)
)

func init() {
	install.Install(groupFactoryRegistry, registry, Scheme)

	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})

	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	// SharedInformerFactory provides shared informers for resources
	SharedInformerFactory informers.SharedInformerFactory
}

// PagerServer contains state for a Kubernetes cluster master/api server.
type PagerServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	*Config
	completedConfig *genericapiserver.CompletedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (c *Config) Complete() completedConfig {
	completedCfg := c.GenericConfig.Complete()

	c.GenericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	return completedConfig{Config: c, completedConfig: &completedCfg}
}

// SkipComplete provides a way to construct a server instance without config completion.
func (c *Config) SkipComplete() completedConfig {
	completedCfg := c.GenericConfig.Complete()
	return completedConfig{Config: c, completedConfig: &completedCfg}
}

// New returns a new instance of PagerServer from the given config.
func (c completedConfig) New() (*PagerServer, error) {
	genericServer, err := c.completedConfig.New("pager", genericapiserver.EmptyDelegate) // completion is done in Complete, no need for a second time
	if err != nil {
		return nil, err
	}

	s := &PagerServer{
		GenericAPIServer: genericServer,
	}

	// create a new api group structure
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(pager.GroupName, registry, Scheme, metav1.ParameterCodec, Codecs)
	// set the preferred version for the group
	apiGroupInfo.GroupMeta.GroupVersion = v1alpha1.SchemeGroupVersion

	// create a REST storage backend for alerts
	alertStorage, alertStatusStorage, err := alertstorage.NewREST(Scheme, c.GenericConfig.RESTOptionsGetter)
	if err != nil {
		return nil, err
	}

	// create a rest mapper for v1alpha1 resources
	v1alpha1storage := map[string]rest.Storage{}
	v1alpha1storage["alerts"] = alertStorage
	v1alpha1storage["alerts/status"] = alertStatusStorage
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = v1alpha1storage

	// create a rest mapper for v1beta1 resources
	v1beta1storage := map[string]rest.Storage{}
	v1beta1storage["alerts"] = alertStorage
	v1beta1storage["alerts/status"] = alertStatusStorage
	apiGroupInfo.VersionedResourcesStorageMap["v1beta1"] = v1beta1storage

	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}
