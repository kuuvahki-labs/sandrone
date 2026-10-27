package service

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) parseRemoteBatch(ctx context.Context, format string, batch domain.RemoteBatchInput) (*parseInputResult, error) {
	if len(batch.Sources) == 0 || batch.Concurrency < 0 || batch.Concurrency > domain.MaxRemoteBatchConcurrency {
		return nil, domain.NewError(domain.CodeInvalidArgument, "remote_batch requires sources and concurrency between 0 and 32")
	}
	concurrency := batch.Concurrency
	if concurrency == 0 {
		concurrency = domain.DefaultRemoteBatchConcurrency
	}
	concurrency = min(concurrency, len(batch.Sources))
	results := make([]domain.RemoteSourceResult, len(batch.Sources))
	parsed := make([]*parseInputResult, len(batch.Sources))
	for index, input := range batch.Sources {
		sourceURL := batchSourceURL(input.URL)
		results[index] = domain.RemoteSourceResult{Index: index, URL: sourceURL, FetchStatus: "skipped", ParseStatus: "skipped", Error: batchSourceError(context.Canceled, "remote source canceled before fetch", sourceURL, domain.CodeRemoteFetchCanceled)}
	}
	var workers sync.WaitGroup
	jobs := make(chan int)
	for range concurrency {
		workers.Go(func() {
			for index := range jobs {
				if ctx.Err() != nil {
					continue
				}
				input := batch.Sources[index]
				result := domain.RemoteSourceResult{Index: index, URL: batchSourceURL(input.URL), FetchStatus: "failed", ParseStatus: "skipped"}
				remote, err := s.fetchRemoteInput(ctx, input)
				if err != nil {
					result.Error = batchSourceError(err, "remote source fetch failed", result.URL, domain.CodeRemoteFetchFailed)
					results[index] = result
					continue
				}
				result.FetchStatus = "ok"
				remote.SourceRef.Name, remote.SourceRef.URL = result.URL, result.URL
				var item *parseInputResult
				if isAutoNodeFormat(format) && isRemoteSubscriptionURLList(remote.Body) {
					err = domain.NewError(domain.CodeParseFailed, "remote source contains subscription URLs instead of nodes")
				} else {
					item, err = s.parseNodeContent(ctx, format, remote.Body, true, &remote.SourceRef)
				}
				if err == nil && len(item.Nodes) == 0 {
					err = domain.NewError(domain.CodeParseFailed, "remote source contained no nodes")
				}
				if err != nil {
					result.ParseStatus = "failed"
					result.Error = batchSourceError(err, "remote source parse failed", result.URL, domain.CodeParseFailed)
				} else {
					result.ParseStatus = "ok"
					result.NodeCount = len(item.Nodes)
					if item.Source != nil {
						result.Format = item.Source.Format
					}
					parsed[index] = item
				}
				results[index] = result
			}
		})
	}
dispatch:
	for index := range batch.Sources {
		if ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
			break dispatch
		case jobs <- index:
		}
	}
	close(jobs)
	workers.Wait()
	out := &parseInputResult{SourceResults: results, Status: "ok", Source: &domain.SourceInfo{}}
	for index, item := range parsed {
		if results[index].FetchStatus == "skipped" && ctx.Err() != nil {
			results[index].Error = batchSourceError(ctx.Err(), "remote source canceled before fetch", results[index].URL, domain.CodeRemoteFetchCanceled)
		}
		if item == nil {
			out.Status = "partial"
			out.Source.Warnings = append(out.Source.Warnings, domain.Warning{Code: "remote_source_failed", Message: "remote source failed; see source_results", Source: results[index].URL})
			continue
		}
		out.Nodes = append(out.Nodes, item.Nodes...)
		if item.Source != nil {
			out.Source.SourceRefs = append(out.Source.SourceRefs, item.Source.SourceRefs...)
			out.Source.Warnings = append(out.Source.Warnings, item.Source.Warnings...)
		}
	}
	if ctx.Err() != nil {
		out.Status = "failed"
		return out, ctx.Err()
	}
	if len(out.Nodes) == 0 {
		out.Status = "failed"
		return out, domain.NewError(domain.CodeParseFailed, "remote batch produced no nodes")
	}
	return out, nil
}

func batchSourceURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[invalid URL]"
	}
	parsed.User, parsed.RawQuery, parsed.Fragment, parsed.RawFragment = nil, "", "", ""
	parsed.ForceQuery = false
	return parsed.String()
}

func batchSourceError(err error, message, source string, code domain.ErrorCode) *domain.AppError {
	if appErr, ok := errors.AsType[*domain.AppError](err); ok {
		code = appErr.Code
	}
	if errors.Is(err, context.Canceled) {
		code, message = domain.CodeRemoteFetchCanceled, "remote source fetch canceled"
	} else if errors.Is(err, context.DeadlineExceeded) {
		code, message = domain.CodeRemoteFetchTimeout, "remote source fetch timed out"
	} else if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
		code, message = domain.CodeRemoteFetchTimeout, "remote source fetch timed out"
	}
	return &domain.AppError{Code: code, Message: message, Source: source}
}

// Only batch auto-detection rejects obvious URL directories. Explicit URI input
// retains the existing HTTP proxy syntax, and no remote body is fetched recursively.
func isRemoteSubscriptionURLList(body []byte) bool {
	for _, candidate := range autoSubscriptionContentCandidates(body) {
		found, onlySubscriptionURLs := false, true
		for _, line := range strings.Split(string(candidate.content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parsed, err := url.Parse(line)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
				onlySubscriptionURLs = false
				break
			}
			if parsed.Port() != "" && parsed.Path == "" && parsed.RawQuery == "" && !parsed.ForceQuery {
				onlySubscriptionURLs = false
				break
			}
			found = true
		}
		if found && onlySubscriptionURLs {
			return true
		}
	}
	return false
}
