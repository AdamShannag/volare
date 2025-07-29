package populator

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	"github.com/AdamShannag/volare/pkg/utils"
	"github.com/AdamShannag/volare/pkg/workerpool"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"log/slog"
	"path/filepath"
)

func ArgsFactory(mountPath string) func(_ bool, u *unstructured.Unstructured) ([]string, error) {
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

	jobProcessor := func(ctx context.Context, src types.Source) error {
		fetcherInstance, errGet := registry.Get(src.Type)
		if errGet != nil {
			return errGet
		}
		return fetcherInstance.Fetch(ctx, filepath.Join(mountPath, src.TargetPath), src)
	}

	numOfWorkers := types.DefaultNumberOfWorkers
	if spec.Workers != nil {
		numOfWorkers = *spec.Workers
	}

	pool := workerpool.New(ctx, numOfWorkers, len(spec.Sources), jobProcessor)

	pool.Start()

	for _, src := range spec.Sources {
		if err = pool.Submit(src); err != nil {
			pool.Cancel()
			return fmt.Errorf("failed to submit job: %w", err)
		}
	}

	pool.Stop()

	for err = range pool.Errors() {
		if err != nil {
			return err
		}
	}

	return nil
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
