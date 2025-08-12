package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/AdamShannag/volare/internal/populator"
	"github.com/AdamShannag/volare/pkg/cloner"
	"github.com/AdamShannag/volare/pkg/downloader"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/fetcher/gcs"
	"github.com/AdamShannag/volare/pkg/fetcher/git"
	"github.com/AdamShannag/volare/pkg/fetcher/github"
	"github.com/AdamShannag/volare/pkg/fetcher/gitlab"
	httpf "github.com/AdamShannag/volare/pkg/fetcher/http"
	"github.com/AdamShannag/volare/pkg/fetcher/s3"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"

	"github.com/kubernetes-csi/lib-volume-populator/populator-machinery"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/lmittmann/tint"
)

const populatorTimeout = 30 * time.Second

func main() {
	var (
		masterURL    string
		kubeconfig   string
		image        string
		httpEndpoint string
		metricsPath  string
		namespace    string
		prefix       string
		mountPath    string
		devicePath   string
		mode         string
		group        string
		kind         string
		groupVersion string
		resource     string
		spec         string
		envs         string
		resources    string
		resourcesMap string
	)

	flag.StringVar(&masterURL, "masterurl", "", "Kubernetes API server URL (optional, in-cluster if empty)")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file (optional, in-cluster if empty)")
	flag.StringVar(&image, "image", "", "Image for populator component (required)")
	flag.StringVar(&httpEndpoint, "httpendpoint", "", "SourceTypeHTTP endpoint for populator (optional)")
	flag.StringVar(&metricsPath, "metricspath", "", "Metrics endpoint path (optional)")
	flag.StringVar(&namespace, "namespace", "", "Namespace for populator component (required)")
	flag.StringVar(&prefix, "prefix", "k8s.volare.dev", "Resource prefix")
	flag.StringVar(&mountPath, "mountpath", "/mnt/vol", "Mount path to PVC")
	flag.StringVar(&devicePath, "devicepath", "", "Device path (optional)")
	flag.StringVar(&mode, "mode", "", "Mode to run: controller or populator (required)")
	flag.StringVar(&group, "group", "k8s.volare.dev", "API group")
	flag.StringVar(&kind, "kind", "VolarePopulator", "Kind name")
	flag.StringVar(&groupVersion, "groupversion", "v1alpha1", "API group version")
	flag.StringVar(&resource, "resource", "volarepopulators", "Resource name")
	flag.StringVar(&spec, "spec", "", "JSON Specs passed to the populator")
	flag.StringVar(&envs, "envs", "", "JSON Envs passed to the populator")
	flag.StringVar(&resources, "resources", "", "Path to a directory containing external files (e.g., credentials) to be passed to the populator")
	flag.StringVar(&resourcesMap, "resourcesMap", "", "Base64-encoded JSON map of additional resource files to pass to the populator")

	flag.Parse()

	logger := slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			AddSource:  true,
			Level:      slog.LevelInfo,
			TimeFormat: time.DateTime,
		}),
	)

	slog.SetDefault(logger)

	switch mode {
	case "controller":
		gk := schema.GroupKind{
			Group: group,
			Kind:  kind,
		}
		gvr := schema.GroupVersionResource{
			Group:    group,
			Version:  groupVersion,
			Resource: resource,
		}

		populator_machinery.RunController(
			masterURL,
			kubeconfig,
			image,
			httpEndpoint,
			metricsPath,
			namespace,
			prefix,
			gk,
			gvr,
			mountPath,
			devicePath,
			populator.ArgsFactory(mountPath, resources),
		)

	case "populator":
		err := utils.LoadEnvFromJSON([]byte(envs))
		if err != nil {
			log.Fatal(err)
		}

		if resourcesMap != "" {
			if err = utils.WriteResourcesDir(resourcesMap, types.ResourcesDir); err != nil {
				log.Fatal(err)
			}
			defer func() {
				if rmErr := os.RemoveAll(types.ResourcesDir); rmErr != nil {
					slog.Error("failed to remove resources directory", "error", rmErr)
				}
			}()
		}

		httpClient := &http.Client{Timeout: populatorTimeout}
		httpDownloader := downloader.NewHTTPDownloader(downloader.WithHTTPClient(httpClient))

		registry := fetcher.NewRegistry()
		err = registry.RegisterAll([]fetcher.RegistryItem{
			fetcher.NewRegistryItem(types.SourceTypeHTTP, httpf.NewFetcher(httpDownloader, WithLogger(logger, types.SourceTypeHTTP))),
			fetcher.NewRegistryItem(types.SourceTypeGITLAB, gitlab.NewFetcher(httpDownloader, WithLogger(logger, types.SourceTypeGITLAB), gitlab.WithHTTPClient(httpClient))),
			fetcher.NewRegistryItem(types.SourceTypeGITHUB, github.NewFetcher(httpDownloader, WithLogger(logger, types.SourceTypeGITHUB), github.WithHTTPClient(httpClient))),
			fetcher.NewRegistryItem(types.SourceTypeS3, s3.NewFetcher(s3.MinioClientFactory, WithLogger(logger, types.SourceTypeS3))),
			fetcher.NewRegistryItem(types.SourceTypeGIT, git.NewFetcher(cloner.NewGitClonerFactory(), WithLogger(logger, types.SourceTypeGIT))),
			fetcher.NewRegistryItem(types.SourceTypeGCS, gcs.NewFetcher(gcs.GCSClientFactory, WithLogger(logger, types.SourceTypeGCS))),
		})

		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), populatorTimeout)
		defer cancel()

		err = populator.Populate(ctx, spec, mountPath, registry)
		if err != nil {
			log.Fatal(err)
		}

	default:
		log.Fatalf("mode [%q] is not supported", mode)
	}
}

func WithLogger(logger *slog.Logger, sourceType types.SourceType) *slog.Logger {
	return logger.With("source", sourceType)
}
