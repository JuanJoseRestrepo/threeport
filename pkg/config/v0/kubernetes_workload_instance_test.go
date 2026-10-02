package v0

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiserver_lib "github.com/threeport/threeport/pkg/api-server/lib/v0"
	api_v0 "github.com/threeport/threeport/pkg/api/v0"
	util "github.com/threeport/threeport/pkg/util/v0"
)

// TestKubernetesWorkloadInstanceConfig_Create_WithoutStatus is a regression
// test for a crash in `tptctl create kubernetes-workload`.
//
// The config built from the API's response dereferenced the created
// instance's Status.  An instance that was just created has no status yet -
// the controller sets one when it reconciles - so the pointer is nil and
// every create segfaulted, taking the command down before it could report
// what it had done.
func TestKubernetesWorkloadInstanceConfig_Create_WithoutStatus(t *testing.T) {
	definitionID := uint(1)
	definitionName := "wordpress-def"
	instanceID := uint(2)
	instanceName := "wordpress-inst"
	runtimeInstanceID := uint(3)
	runtimeInstanceName := "threeport-test"
	yamlDocument := `apiVersion: v1
kind: ConfigMap
metadata:
  name: some-config
data:
  key: value
`

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data []apiserver_lib.Object

		switch {
		case strings.Contains(r.URL.Path, "kubernetes-runtime-instances"):
			data = []apiserver_lib.Object{api_v0.KubernetesRuntimeInstance{
				Common:         api_v0.Common{ID: &runtimeInstanceID},
				Instance:       api_v0.Instance{Name: &runtimeInstanceName},
				DefaultRuntime: util.Ptr(true),
			}}
		case strings.Contains(r.URL.Path, "kubernetes-workload-definitions"):
			data = []apiserver_lib.Object{api_v0.KubernetesWorkloadDefinition{
				Common:       api_v0.Common{ID: &definitionID},
				Definition:   api_v0.Definition{Name: &definitionName},
				YAMLDocument: &yamlDocument,
			}}
		case strings.Contains(r.URL.Path, "kubernetes-workload-instances"):
			// the created instance comes back with no status, which is what
			// the API returns before anything has reconciled it
			data = []apiserver_lib.Object{api_v0.KubernetesWorkloadInstance{
				Common:                         api_v0.Common{ID: &instanceID},
				Instance:                       api_v0.Instance{Name: &instanceName},
				KubernetesWorkloadDefinitionID: &definitionID,
			}}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		statusCode := http.StatusOK
		if r.Method == http.MethodPost {
			statusCode = http.StatusCreated
		}
		body, err := json.Marshal(apiserver_lib.Response{
			Status: apiserver_lib.Status{Code: statusCode, Message: http.StatusText(statusCode)},
			Data:   data,
		})
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write(body)
	}))
	defer apiServer.Close()

	workloadInstanceConfig := KubernetesWorkloadInstanceConfig{
		KubernetesWorkloadInstance: KubernetesWorkloadInstanceValues{
			Name: util.Ptr(instanceName),
			KubernetesWorkloadDefinition: &KubernetesWorkloadDefinitionValues{
				Name: util.Ptr(definitionName),
			},
		},
	}

	created, err := workloadInstanceConfig.Create(
		&http.Client{},
		strings.TrimPrefix(apiServer.URL, "http://"),
	)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, instanceName, *created.KubernetesWorkloadInstance.Name)
	// a missing status reads as empty rather than taking the process down
	assert.Equal(t, "", *created.KubernetesWorkloadInstance.Status)
}
