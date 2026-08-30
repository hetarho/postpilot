package postpilot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"connectrpc.com/connect"
	postpilotv1 "github.com/postpilot/agent/internal/gen/postpilot/v1"
	"github.com/postpilot/agent/internal/gen/postpilot/v1/postpilotv1connect"
)

type Client struct {
	agent postpilotv1connect.PublishingAgentServiceClient
}

func New(apiURL, token string) *Client {
	origin, _ := url.Parse(strings.TrimRight(apiURL, "/"))
	httpClient := &http.Client{
		Timeout:       30 * time.Second,
		Transport:     bearerTransport{token: token, origin: origin, base: http.DefaultTransport},
		CheckRedirect: rejectRedirect,
	}
	return &Client{agent: postpilotv1connect.NewPublishingAgentServiceClient(httpClient, strings.TrimRight(apiURL, "/"))}
}

func Enroll(ctx context.Context, apiURL, code, browserLabel string) (*postpilotv1.EnrollPublishingAgentResponse, error) {
	client := postpilotv1connect.NewPublishingAgentServiceClient(&http.Client{Timeout: 30 * time.Second, CheckRedirect: rejectRedirect}, strings.TrimRight(apiURL, "/"))
	response, err := client.EnrollPublishingAgent(ctx, connect.NewRequest(&postpilotv1.EnrollPublishingAgentRequest{DeviceCode: code, BrowserLabel: browserLabel}))
	if err != nil {
		return nil, err
	}
	return response.Msg, nil
}

func (c *Client) SyncProfile(ctx context.Context, request *postpilotv1.SyncAgentProfileRequest) (*postpilotv1.PublishingAgent, error) {
	response, err := c.agent.SyncAgentProfile(ctx, connect.NewRequest(request))
	if err != nil {
		return nil, err
	}
	return response.Msg.Agent, nil
}

func (c *Client) Claim(ctx context.Context) (*postpilotv1.ClaimPublishJobResponse, error) {
	response, err := c.agent.ClaimPublishJob(ctx, connect.NewRequest(&postpilotv1.ClaimPublishJobRequest{}))
	if err != nil {
		return nil, err
	}
	return response.Msg, nil
}

func (c *Client) Renew(ctx context.Context, jobID, leaseToken string) error {
	_, err := c.agent.RenewPublishLease(ctx, connect.NewRequest(&postpilotv1.RenewPublishLeaseRequest{JobId: jobID, LeaseToken: leaseToken}))
	return err
}

func (c *Client) Progress(ctx context.Context, jobID, leaseToken string, sequence int64, stage postpilotv1.PublishStage) error {
	_, err := c.agent.ReportPublishProgress(ctx, connect.NewRequest(&postpilotv1.ReportPublishProgressRequest{JobId: jobID, LeaseToken: leaseToken, ProgressSequence: sequence, Stage: stage}))
	return err
}

func (c *Client) Complete(ctx context.Context, jobID, leaseToken string, sequence int64, publishedURL string) error {
	_, err := c.agent.CompletePublish(ctx, connect.NewRequest(&postpilotv1.CompletePublishRequest{JobId: jobID, LeaseToken: leaseToken, ProgressSequence: sequence, PlatformPostUrl: publishedURL}))
	return err
}

func (c *Client) Fail(ctx context.Context, jobID, leaseToken string, sequence int64, kind postpilotv1.PublishFailureKind, detail string) error {
	_, err := c.agent.FailPublish(ctx, connect.NewRequest(&postpilotv1.FailPublishRequest{JobId: jobID, LeaseToken: leaseToken, ProgressSequence: sequence, Kind: kind, Detail: detail}))
	return err
}

type bearerTransport struct {
	token  string
	origin *url.URL
	base   http.RoundTripper
}

func (t bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.origin == nil || !sameOrigin(t.origin, request.URL) {
		return nil, fmt.Errorf("refusing to send agent credential outside configured API origin")
	}
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

func sameOrigin(expected, actual *url.URL) bool {
	return strings.EqualFold(expected.Scheme, actual.Scheme) && strings.EqualFold(expected.Host, actual.Host)
}

func rejectRedirect(*http.Request, []*http.Request) error {
	return errors.New("Postpilot API redirects are disabled")
}
