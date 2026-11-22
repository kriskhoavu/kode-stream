package jira

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"kode-stream/internal/common/models"
)

type AttachmentContent struct {
	Body           io.ReadCloser
	MediaType      string
	Filename       string
	DeclaredLength int64
}

const MaxAttachmentBytes int64 = 25 * 1024 * 1024

func (c *Client) GetAttachment(ctx context.Context, connection models.JiraConnection, attachment Attachment) (AttachmentContent, error) {
	target, err := url.Parse(attachment.ContentURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return AttachmentContent{}, errors.New("Jira attachment URL is invalid")
	}
	base, _ := url.Parse(connection.BaseURL)
	if !strings.EqualFold(target.Scheme, base.Scheme) || !strings.EqualFold(target.Host, base.Host) {
		return AttachmentContent{}, errors.New("Jira attachment URL changed origin")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return AttachmentContent{}, err
	}
	if err := c.authorize(request, connection); err != nil {
		return AttachmentContent{}, err
	}
	attachmentClient := *c.httpClient
	attachmentClient.CheckRedirect = attachmentRedirectPolicy(connection)
	response, err := attachmentClient.Do(request)
	if err != nil {
		return AttachmentContent{}, fmt.Errorf("Jira attachment is unavailable: %w", err)
	}
	if err := responseError(response, errors.New("Jira attachment was not found")); err != nil {
		return AttachmentContent{}, err
	}
	if response.ContentLength > MaxAttachmentBytes {
		response.Body.Close()
		return AttachmentContent{}, errors.New("Jira attachment exceeds the size limit")
	}
	mediaType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	if mediaType == "" {
		mediaType = attachment.MediaType
	}
	return AttachmentContent{Body: response.Body, MediaType: mediaType, Filename: attachment.Filename, DeclaredLength: response.ContentLength}, nil
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

func attachmentRedirectPolicy(connection models.JiraConnection) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return nil
		}
		if len(via) >= 5 {
			return errors.New("too many Jira redirects")
		}
		if strings.EqualFold(request.URL.Scheme, via[0].URL.Scheme) && strings.EqualFold(request.URL.Host, via[0].URL.Host) {
			return nil
		}
		trustedCloudMedia := strings.EqualFold(connection.DeploymentType, "cloud") &&
			strings.EqualFold(request.URL.Scheme, "https") &&
			strings.EqualFold(request.URL.Hostname(), "api.media.atlassian.com") &&
			request.URL.Port() == ""
		if !trustedCloudMedia {
			return errors.New("Jira redirect changed origin")
		}
		request.Header.Del("Authorization")
		request.Header.Del("Cookie")
		request.Header.Del("Proxy-Authorization")
		return nil
	}
}
