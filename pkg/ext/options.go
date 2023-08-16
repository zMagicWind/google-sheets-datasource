package ext

import (
	"fmt"
	"io"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	clientGoInformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"github.com/grafana/google-sheets-datasource/pkg/apis/googlesheets/v1"
	"github.com/grafana/google-sheets-datasource/pkg/client/clientset/clientset"
	"github.com/grafana/google-sheets-datasource/pkg/client/clientset/clientset/scheme"
	informers "github.com/grafana/google-sheets-datasource/pkg/client/informers/externalversions"

	generatedopenapi "github.com/grafana/google-sheets-datasource/pkg/client/openapi"
	"k8s.io/apiserver/pkg/authentication/user"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/openapi"
	netutils "k8s.io/utils/net"
	"net"
)

type PluginAggregatedServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	SharedInformerFactory informers.SharedInformerFactory
	StdOut                io.Writer
	StdErr                io.Writer

	AlternateDNS []string
}

func NewPluginAggregatedServerOptions(out, errOut io.Writer) *PluginAggregatedServerOptions {
	o := &PluginAggregatedServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			"",
			Codecs.LegacyCodec(v1.SchemeGroupVersion),
		),
		StdOut: out,
		StdErr: errOut,
	}
	o.RecommendedOptions.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(v1.SchemeGroupVersion, schema.GroupKind{Group: v1.GroupName})
	return o
}

// Complete fills in fields required to have valid data
func (o *PluginAggregatedServerOptions) Complete() error {
	return nil
}

func (o *PluginAggregatedServerOptions) Config() (*Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", o.AlternateDNS, []net.IP{netutils.ParseIPSloppy("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	o.RecommendedOptions.ExtraAdmissionInitializers = func(c *genericapiserver.RecommendedConfig) ([]admission.PluginInitializer, error) {
		client, err := clientset.NewForConfig(c.LoopbackClientConfig)
		if err != nil {
			return nil, err
		}
		informerFactory := informers.NewSharedInformerFactory(client, c.LoopbackClientConfig.Timeout)
		o.SharedInformerFactory = informerFactory
		return []admission.PluginInitializer{}, nil
	}

	o.RecommendedOptions.SecureServing.BindPort = 6443
	// o.RecommendedOptions.Authentication.DisableAnonymous = false
	o.RecommendedOptions.Authentication.RemoteKubeConfigFile = "/Users/charandas/.kube/config"
	o.RecommendedOptions.Authorization.RemoteKubeConfigFile = "/Users/charandas/.kube/config"
	o.RecommendedOptions.Authorization.AlwaysAllowPaths = []string{"*"}
	o.RecommendedOptions.Authorization.AlwaysAllowGroups = []string{user.Anonymous}
	o.RecommendedOptions.Etcd = nil
	o.RecommendedOptions.CoreAPI = nil

	serverConfig := genericapiserver.NewRecommendedConfig(Codecs)
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openapi.GetOpenAPIDefinitionsWithoutDisabledFeatures(generatedopenapi.GetOpenAPIDefinitions), openapinamer.NewDefinitionNamer(Scheme, scheme.Scheme))
	serverConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(openapi.GetOpenAPIDefinitionsWithoutDisabledFeatures(generatedopenapi.GetOpenAPIDefinitions), openapinamer.NewDefinitionNamer(Scheme, scheme.Scheme))
	serverConfig.SkipOpenAPIInstallation = false
	serverConfig.SharedInformerFactory = clientGoInformers.NewSharedInformerFactory(fake.NewSimpleClientset(), 10*time.Minute)
	serverConfig.ClientConfig = &rest.Config{}

	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	config := &Config{
		GenericConfig: serverConfig,
	}
	return config, nil
}
