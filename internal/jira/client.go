package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"plan-manager/internal/models"
)

type ConnectionTest struct {
	OK             bool   `json:"ok"`
	DeploymentType string `json:"deploymentType"`
	ProjectKey     string `json:"projectKey"`
	Message        string `json:"message"`
	RecoveryHint   string `json:"recoveryHint,omitempty"`
}

type Client struct {
	httpClient *http.Client
	getenv     func(string) string
}

func New() *Client {
	return &Client{httpClient: &http.Client{Timeout: 12 * time.Second, CheckRedirect: sameOriginRedirect}, getenv: os.Getenv}
}

func (c *Client) TestConnection(ctx context.Context, connection models.JiraConnection) (ConnectionTest, error) {
	token := strings.TrimSpace(c.getenv(connection.TokenEnvVar))
	if token == "" {
		return ConnectionTest{}, fmt.Errorf("Jira token environment variable %s is not available", connection.TokenEnvVar)
	}
	version := "2"
	if connection.DeploymentType == "cloud" {
		version = "3"
	}
	for _, endpoint := range []string{"myself", "project/" + url.PathEscape(connection.ProjectKey)} {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, connection.BaseURL+"/rest/api/"+version+"/"+endpoint, nil)
		if err != nil {
			return ConnectionTest{}, err
		}
		request.Header.Set("Accept", "application/json")
		if connection.DeploymentType == "cloud" {
			request.SetBasicAuth(connection.AccountEmail, token)
		} else {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := c.httpClient.Do(request)
		if err != nil {
			return ConnectionTest{}, fmt.Errorf("Jira is unavailable: %w", err)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		response.Body.Close()
		if response.StatusCode == http.StatusUnauthorized {
			return ConnectionTest{}, errors.New("Jira authentication failed")
		}
		if response.StatusCode == http.StatusForbidden {
			return ConnectionTest{}, errors.New("Jira access is forbidden")
		}
		if response.StatusCode == http.StatusNotFound && strings.HasPrefix(endpoint, "project/") {
			return ConnectionTest{}, errors.New("Jira project was not found")
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return ConnectionTest{}, fmt.Errorf("Jira returned status %d", response.StatusCode)
		}
	}
	return ConnectionTest{OK: true, DeploymentType: connection.DeploymentType, ProjectKey: connection.ProjectKey, Message: "Jira connection succeeded"}, nil
}

func sameOriginRedirect(request *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	if !strings.EqualFold(request.URL.Scheme, via[0].URL.Scheme) || !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
		return errors.New("Jira redirect changed origin")
	}
	if len(via) >= 5 {
		return errors.New("too many Jira redirects")
	}
	return nil
}

func decodeBounded(response *http.Response, target any) error {
	defer response.Body.Close()
	return json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(target)
}
