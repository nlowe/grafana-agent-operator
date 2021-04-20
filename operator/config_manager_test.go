package operator

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grafana/agent/pkg/prom/instance"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNoOpConfigManager(t *testing.T) {
	sut := NewNoOpConfigManager()

	assert.NoError(t, sut.DeleteScrapeConfig(nil))
	assert.NoError(t, sut.UpdateScrapeConfig(nil))
}

func makeMockAgentServerWithBody(code int, body string) (*string, *httptest.Server, *grafanaAgentConfigManager) {
	var path string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))

	return &path, server, NewGrafanaAgentConfigManager(server.URL)
}

func makeMockAgentServer(code int) (*string, *httptest.Server, *grafanaAgentConfigManager) {
	return makeMockAgentServerWithBody(code, "")
}

func TestRoute(t *testing.T) {
	sut := NewGrafanaAgentConfigManager("http://agent.monitoring.cluster.svc.local:8888/")

	tests := []struct {
		name     string
		expected string
	}{
		{name: "dummy", expected: "http://agent.monitoring.cluster.svc.local:8888/agent/api/v1/config/dummy"},
		{name: "foo/bar", expected: "http://agent.monitoring.cluster.svc.local:8888/agent/api/v1/config/foo%2Fbar"},
		{name: "foo/bar/baz", expected: "http://agent.monitoring.cluster.svc.local:8888/agent/api/v1/config/foo%2Fbar%2Fbaz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, sut.route(&instance.Config{Name: tt.name}))
		})
	}
}

func TestGrafanaAgentConfigManager(t *testing.T) {
	logrus.SetOutput(ioutil.Discard)

	cfg := &instance.Config{Name: "dummy"}

	t.Run("ListScrapeConfig", func(t *testing.T) {
		path, server, sut := makeMockAgentServerWithBody(http.StatusOK, `{
  "status": "success",
  "data": {
    "configs": [
      "a",
      "b",
      "c"
    ]
  }
}`)
		defer server.Close()

		cfgs, err := sut.ListScrapeConfigs()
		assertResponse(t, path, "/agent/api/v1/configs", nil, err)

		assert.Len(t, cfgs, 3)
		assert.Contains(t, cfgs, "a")
		assert.Contains(t, cfgs, "b")
		assert.Contains(t, cfgs, "c")
	})

	t.Run("UpdateScrapeConfig", func(t *testing.T) {
		tests := []struct {
			name     string
			code     int
			expected error
		}{
			{name: "Created", code: http.StatusCreated},
			{name: "Updated", code: http.StatusOK},
			{
				name:     "Error",
				code:     http.StatusInternalServerError,
				expected: fmt.Errorf("UpdateScrapeConfig: unexpected status code: 500 Internal Server Error"),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				path, server, sut := makeMockAgentServer(tt.code)
				defer server.Close()

				err := sut.UpdateScrapeConfig(cfg)
				assertResponse(t, path, "/agent/api/v1/config/dummy", tt.expected, err)
			})
		}
	})

	t.Run("DeleteScrapeConfig", func(t *testing.T) {
		tests := []struct {
			name     string
			code     int
			expected error
		}{
			{name: "Deleted", code: http.StatusOK},
			{name: "Bad Name", code: http.StatusBadRequest},
			{
				name:     "Error",
				code:     http.StatusInternalServerError,
				expected: fmt.Errorf("DeleteScrapeConfig: unexpected status code: 500 Internal Server Error"),
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				path, server, sut := makeMockAgentServer(tt.code)
				defer server.Close()

				err := sut.DeleteScrapeConfig(cfg)
				assertResponse(t, path, "/agent/api/v1/config/dummy", tt.expected, err)
			})
		}
	})
}

func assertResponse(t *testing.T, path *string, expectedPath string, expected, err error) {
	assert.Equal(t, *path, expectedPath)

	if expected != nil {
		require.EqualError(t, err, expected.Error())
	} else {
		require.NoError(t, err)
	}
}
