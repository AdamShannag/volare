package populator_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/AdamShannag/volare/internal/populator"
	"github.com/AdamShannag/volare/pkg/fetcher"
	"github.com/AdamShannag/volare/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mockFetcher struct {
	mu       sync.Mutex
	Called   []types.Source
	Fail     bool
	FetchErr error
}

func (m *mockFetcher) Fetch(_ context.Context, _ string, src types.Source) (*fetcher.Object, error) {
	m.mu.Lock()
	m.Called = append(m.Called, src)
	m.mu.Unlock()

	if m.Fail {
		return nil, m.FetchErr
	}
	return &fetcher.Object{}, nil
}

func TestPopulate_Success(t *testing.T) {
	t.Parallel()

	reg := fetcher.NewRegistry()
	mock := &mockFetcher{}
	_ = reg.Register("s3", mock)

	spec := newValidSpec(t)
	err := populator.Populate(context.Background(), spec, "/tmp/populate", reg)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if len(mock.Called) != 2 {
		t.Errorf("expected 2 fetch calls, got: %d", len(mock.Called))
	}
}

func TestPopulate_FetcherGetFails(t *testing.T) {
	t.Parallel()

	reg := fetcher.NewRegistry()

	spec := newValidSpec(t)
	err := populator.Populate(context.Background(), spec, "/tmp", reg)
	if err == nil {
		t.Fatal("expected fetcher not found error, got nil")
	}
}

func TestPopulate_FetchFails(t *testing.T) {
	t.Parallel()

	reg := fetcher.NewRegistry()
	mock := &mockFetcher{Fail: true, FetchErr: errors.New("boom")}
	_ = reg.Register("s3", mock)

	spec := newValidSpec(t)
	err := populator.Populate(context.Background(), spec, "/tmp", reg)
	if err == nil {
		t.Fatal("expected fetch error, got nil")
	}
}

func TestPopulate_InvalidSpecJSON(t *testing.T) {
	t.Parallel()

	err := populator.Populate(context.Background(), `{"invalid":`, "/tmp", nil)
	if err == nil {
		t.Fatal("expected JSON error, got nil")
	}
}

func TestPopulate_EmptySpec(t *testing.T) {
	t.Parallel()

	err := populator.Populate(context.Background(), "", "/tmp", nil)
	if err == nil {
		t.Fatal("expected error for empty spec, got nil")
	}
}

func TestArgsFactory_Success(t *testing.T) {
	t.Setenv("FOO", "bar")

	vp := types.VolarePopulator{
		TypeMeta:   metav1.TypeMeta{Kind: "VolarePopulator", APIVersion: "volare/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-populator"},
		Spec: types.VolarePopulatorSpec{
			Sources: []types.Source{
				{Type: "http", TargetPath: "path/to/target"},
			},
			Workers: nil,
		},
	}

	unstructuredMap, err := toUnstructured(vp)
	if err != nil {
		t.Fatalf("failed to convert to unstructured: %v", err)
	}
	u := &unstructured.Unstructured{Object: unstructuredMap}

	mountPath := "/mnt/test"
	argsFunc := populator.ArgsFactory(mountPath, "")

	args, err := argsFunc(false, u)
	if err != nil {
		t.Fatalf("ArgsFactory returned error: %v", err)
	}

	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}

	if !strings.HasPrefix(args[0], "--mode=populator") {
		t.Errorf("expected args[0] to start with --mode=populator, got %s", args[0])
	}

	if !strings.HasPrefix(args[1], "--spec=") {
		t.Errorf("expected args[1] to start with --spec=, got %s", args[1])
	}

	if !strings.HasPrefix(args[2], "--envs=") {
		t.Errorf("expected args[2] to start with --envs=, got %s", args[2])
	}

	if !strings.HasPrefix(args[3], "--mountpath=") {
		t.Errorf("expected args[3] to start with --mountpath=, got %s", args[3])
	}

	if args[3] != "--mountpath="+mountPath {
		t.Errorf("expected mountpath arg %q, got %q", "--mountpath="+mountPath, args[3])
	}
}

func TestArgsFactory_InvalidUnstructured(t *testing.T) {
	t.Parallel()

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": func() {},
		},
	}

	argsFunc := populator.ArgsFactory("/mnt/test", "")

	args, err := argsFunc(false, u)
	if err == nil {
		t.Fatal("expected error converting invalid unstructured, got nil")
	}
	if len(args) != 0 {
		t.Errorf("expected empty args slice on error, got %v", args)
	}
}

func TestArgsFactory_WithResources(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	sampleFiles := map[string]string{
		"foo.txt":        "Hello, Foo!",
		"nested/bar.txt": "Nested Bar Content",
	}
	for path, content := range sampleFiles {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write file %s: %v", path, err)
		}
	}

	vp := types.VolarePopulator{
		TypeMeta:   metav1.TypeMeta{Kind: "VolarePopulator", APIVersion: "volare/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-populator"},
		Spec: types.VolarePopulatorSpec{
			Sources: []types.Source{{Type: "http", TargetPath: "path/to/target"}},
		},
	}
	unstructuredMap, err := toUnstructured(vp)
	if err != nil {
		t.Fatalf("failed to convert to unstructured: %v", err)
	}
	u := &unstructured.Unstructured{Object: unstructuredMap}

	mountPath := "/mnt/test"
	argsFunc := populator.ArgsFactory(mountPath, tempDir)

	args, err := argsFunc(false, u)
	if err != nil {
		t.Fatalf("ArgsFactory returned error: %v", err)
	}

	if len(args) < 5 {
		t.Fatalf("expected at least 5 args, got %d: %v", len(args), args)
	}

	var resourcesArg string
	for _, a := range args {
		if strings.HasPrefix(a, "--resourcesMap=") {
			resourcesArg = a
			break
		}
	}
	if resourcesArg == "" {
		t.Fatal("expected --resourcesMap argument but none found")
	}

	jsonStr := strings.TrimPrefix(resourcesArg, "--resourcesMap=")
	var files map[string]string
	if err = json.Unmarshal([]byte(jsonStr), &files); err != nil {
		t.Fatalf("failed to unmarshal resourcesMap JSON: %v", err)
	}

	for k, want := range sampleFiles {
		b64Content, ok := files[k]
		if !ok {
			t.Errorf("missing file key in resourcesMap: %q", k)
			continue
		}
		data, dErr := base64.StdEncoding.DecodeString(b64Content)
		if dErr != nil {
			t.Errorf("failed to decode base64 for key %q: %v", k, dErr)
			continue
		}
		if got := string(data); got != want {
			t.Errorf("content mismatch for %q: got %q, want %q", k, got, want)
		}
	}
}

func toUnstructured(obj interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = json.Unmarshal(b, &m)
	return m, err
}

func newValidSpec(t *testing.T) string {
	spec := types.VolarePopulatorSpec{
		Sources: []types.Source{
			{Type: "s3", TargetPath: "file1.txt"},
			{Type: "s3", TargetPath: "file2.txt"},
		},
	}
	specBytes, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	return string(specBytes)
}
