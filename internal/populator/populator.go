package populator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
	"github.com/AdamShannag/volare/pkg/workerpool"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func ArgsFactory(mountPath, resources string) func(_ bool, u *unstructured.Unstructured) ([]string, error) {
	return func(_ bool, u *unstructured.Unstructured) ([]string, error) {
		var vp types.VolarePopulator
		var args []string

		err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &vp)
		if err != nil {
			slog.Error("failed converting unstructured to VolarePopulator", "error", err)
			return args, err
		}

		specBytes, err := json.Marshal(vp.Spec)
		if err != nil {
			slog.Error("failed to marshal VolarePopulator.Spec to JSON", "error", err)
			return args, err
		}

		envBytes, err := utils.GetEnvJSON()
		if err != nil {
			slog.Error("failed to marshal Envs to JSON", "error", err)
			return args, err
		}

		if resources != "" {
			files, readErr := utils.ReadFilesAsBase64(resources)
			if readErr != nil {
				slog.Error("failed to read external resource files", "path", resources, "error", readErr)
				return args, err
			}

			filesBytes, marshalErr := json.Marshal(files)
			if marshalErr != nil {
				slog.Error("failed to marshal resource files to JSON", "error", marshalErr)
				return args, err
			}

			args = append(args, fmt.Sprintf("--resourcesMap=%s", string(filesBytes)))
		}

		args = append(args, "--mode=populator")
		args = append(args, fmt.Sprintf("--spec=%s", string(specBytes)))
		args = append(args, fmt.Sprintf("--envs=%s", string(envBytes)))
		args = append(args, fmt.Sprintf("--mountpath=%s", mountPath))

		return args, nil
	}
}

func Populate(ctx context.Context, specs string, mountPath string, registry *fetcher.Registry) error {
	spec, err := parseSpecs(specs)
	if err != nil {
		return err
	}

	return workerpool.RunPool(ctx, spec.Sources, spec.Workers, processSource(registry, mountPath))
}

func processSource(registry *fetcher.Registry, mountPath string) func(context.Context, types.Source) error {
	return func(ctx context.Context, src types.Source) error {
		fetcherInstance, err := registry.Get(src.Type)
		if err != nil {
			return fmt.Errorf("get fetcher: %w", err)
		}

		err = validateSourceConfig(src)
		if err != nil {
			return err
		}

		object, err := fetcherInstance.Fetch(ctx, filepath.Join(mountPath, src.TargetPath), src)
		if err != nil {
			return fmt.Errorf("fetch: %w", err)
		}

		defer func() {
			if object != nil && object.Cleanup != nil {
				if cErr := object.Cleanup(ctx); cErr != nil {
					slog.Error("failed to cleanup object", "source", src.Type, "error", cErr)
				}
			}
		}()

		if object == nil {
			return nil
		}

		err = workerpool.RunPool(ctx, object.Objects, object.Workers, object.Processor)
		if err != nil {
			return err
		}

		return err
	}
}

func parseSpecs(specs string) (types.VolarePopulatorSpec, error) {
	if specs == "" {
		return types.VolarePopulatorSpec{}, fmt.Errorf("empty specs string")
	}

	var spec types.VolarePopulatorSpec
	if err := json.Unmarshal([]byte(specs), &spec); err != nil {
		return types.VolarePopulatorSpec{}, fmt.Errorf("failed to unmarshal specs JSON: %w", err)
	}

	return spec, nil
}

func validateSourceConfig(src types.Source) error {
	checks := map[types.SourceType]struct {
		field func(types.Source) any
		label string
	}{
		types.SourceTypeHTTP: {
			field: func(s types.Source) any { return s.Http },
			label: "http",
		},
		types.SourceTypeGITHUB: {
			field: func(s types.Source) any { return s.GitHub },
			label: "github",
		},
		types.SourceTypeGITLAB: {
			field: func(s types.Source) any { return s.Gitlab },
			label: "gitlab",
		},
		types.SourceTypeGIT: {
			field: func(s types.Source) any { return s.Git },
			label: "git",
		},
		types.SourceTypeS3: {
			field: func(s types.Source) any { return s.S3 },
			label: "s3",
		},
		types.SourceTypeGCS: {
			field: func(s types.Source) any { return s.GCS },
			label: "gcs",
		},
	}

	if check, ok := checks[src.Type]; ok {
		if check.field(src) == nil {
			return fmt.Errorf(
				"invalid source configuration: '%s' options must be provided for source type '%s'",
				check.label, strings.ToLower(string(src.Type)),
			)
		}
	}

	return nil
}
