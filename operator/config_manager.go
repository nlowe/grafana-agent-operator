package operator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/grafana/agent/pkg/prom/instance"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/nlowe/grafana-agent-operator/httputil"
	"github.com/sirupsen/logrus"
)

type ConfigManager interface {
	ListScrapeConfigs() ([]string, error)

	UpdateScrapeConfig(cfg *instance.Config) error
	DeleteScrapeConfig(cfg *instance.Config) error
}

type grafanaAgentConfigManager struct {
	apiRoot string
	c       *http.Client

	log logrus.Ext1FieldLogger
}

func NewGrafanaAgentConfigManager(apiRoot string) *grafanaAgentConfigManager {
	return &grafanaAgentConfigManager{
		apiRoot: strings.TrimSuffix(apiRoot, "/"),
		c:       cleanhttp.DefaultPooledClient(),

		log: logrus.WithField("prefix", "configManager"),
	}
}

func (g *grafanaAgentConfigManager) route(cfg *instance.Config) string {
	return fmt.Sprintf("%s/agent/api/v1/config/%s", g.apiRoot, url.PathEscape(cfg.Name))
}

func (g *grafanaAgentConfigManager) ListScrapeConfigs() ([]string, error) {
	type listResponse struct {
		Status string `json:"status"`
		Data   struct {
			Configs []string `json:"configs"`
		} `json:"data"`
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/agent/api/v1/configs", g.apiRoot), nil)
	if err != nil {
		return nil, fmt.Errorf("ListScrapeConfigs: make request: %w", err)
	}

	g.log.Debug("Listing ScrapeConfigs")
	resp, err, dispose := httputil.MakeDisposer(g.c.Do(req))
	defer dispose()

	if err != nil {
		return nil, fmt.Errorf("ListScrapeConfigs: failed to list: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ListScrapeConfigs: unexpected status code: %s", resp.Status)
	}

	payload := listResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("ListScrapeConfigs: unmarshal response: %w", err)
	}

	return payload.Data.Configs, nil
}

func (g *grafanaAgentConfigManager) UpdateScrapeConfig(cfg *instance.Config) error {
	log := g.log.WithField("config", cfg.Name)

	raw, err := instance.MarshalConfig(cfg, false)
	if err != nil {
		return fmt.Errorf("UpdateScrapeConfig: failed to marshal config: %w", err)
	}

	route := g.route(cfg)
	req, err := http.NewRequest(http.MethodPost, route, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("UpdateScrapeConfig: make request: %w", err)
	}

	log.WithField("route", route).Debug("Updating ScrapeConfig")
	resp, err, dispose := httputil.MakeDisposer(g.c.Do(req))
	defer dispose()

	if err != nil {
		return fmt.Errorf("UpdateScrapeConfig: failed to sync: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		log.Info("Config Updated")
	} else if resp.StatusCode == http.StatusCreated {
		log.Info("Config Added")
	} else {
		// TODO: Also dump the body on error?
		return fmt.Errorf("UpdateScrapeConfig: unexpected status code: %s", resp.Status)
	}

	return nil
}

func (g *grafanaAgentConfigManager) DeleteScrapeConfig(cfg *instance.Config) error {
	log := g.log.WithField("config", cfg.Name)

	req, err := http.NewRequest(http.MethodDelete, g.route(cfg), nil)
	if err != nil {
		return fmt.Errorf("DeleteScrapeConfig: create request: %w", err)
	}

	log.Debug("Deleting ScrapeConfig")
	resp, err, dispose := httputil.MakeDisposer(g.c.Do(req))
	defer dispose()

	if err != nil {
		return fmt.Errorf("DeleteScrapeConfig: failed to sync: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		log.Info("Config Deleted")
	} else if resp.StatusCode == http.StatusBadRequest {
		// TODO: How should we handle this? Can we ignore it?
		log.Error("Unknown or invalid config name")
	} else {
		return fmt.Errorf("DeleteScrapeConfig: unexpected status code: %s", resp.Status)
	}

	return nil
}

type noopConfigManager struct{}

func NewNoOpConfigManager() *noopConfigManager {
	return &noopConfigManager{}
}

func (n *noopConfigManager) ListScrapeConfigs() ([]string, error) {
	return nil, nil
}

func (n *noopConfigManager) UpdateScrapeConfig(_ *instance.Config) error {
	return nil
}

func (n *noopConfigManager) DeleteScrapeConfig(_ *instance.Config) error {
	return nil
}
