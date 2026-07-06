package httpapi

import "github.com/kuuvahki-labs/sandrone/internal/domain"

type validateRequest struct {
	File       string                 `json:"file,omitempty"`
	Spec       *domain.FileSpec       `json:"spec,omitempty"`
	Format     string                 `json:"format,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Remote     *domain.RemoteInput    `json:"remote,omitempty"`
	Target     string                 `json:"target,omitempty"`
	Processors []domain.ProcessorSpec `json:"processors,omitempty"`
}

type subscriptionTrafficRequest struct {
	Refresh bool `json:"refresh,omitempty"`
}

type subscriptionPreviewRequest struct {
	Args map[string]string `json:"args,omitempty"`
}

type renderResponse struct {
	ContentType string              `json:"content_type,omitempty"`
	Body        string              `json:"body,omitempty"`
	Response    domain.ResponseInfo `json:"response,omitempty"`
	Warnings    []domain.Warning    `json:"warnings"`
}

type sourceResponse struct {
	ContentType string `json:"content_type,omitempty"`
	Body        string `json:"body"`
}

type validateResponse struct {
	OK       bool                     `json:"ok"`
	Counts   domain.ValidationCounts  `json:"counts"`
	Issues   []domain.ValidationIssue `json:"issues"`
	Warnings []domain.Warning         `json:"warnings"`
}

type inspectResponse struct {
	Capabilities map[string]any `json:"capabilities,omitempty"`
}

type subscriptionPreviewResponse struct {
	SubscriptionName string                               `json:"subscription_name"`
	Type             domain.SubscriptionType              `json:"type,omitempty"`
	Format           string                               `json:"format,omitempty"`
	BeforeCount      int                                  `json:"before_count"`
	AfterCount       int                                  `json:"after_count"`
	StatusCounts     map[string]int                       `json:"status_counts"`
	Nodes            []domain.SubscriptionPreviewNodeDiff `json:"nodes"`
	Warnings         []domain.Warning                     `json:"warnings"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
