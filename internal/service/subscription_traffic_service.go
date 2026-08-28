package service

import (
	"context"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) SubscriptionTraffic(ctx context.Context, req domain.SubscriptionTrafficRequest) (*domain.SubscriptionTrafficResult, error) {
	if s.metaStore == nil {
		return nil, storeUnavailable()
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, domain.NewError(domain.CodeInvalidArgument, "subscription name is required")
	}
	if req.Refresh {
		ctx = withCacheReadBypass(ctx)
	}

	sub, err := s.metaStore.GetSubscription(ctx, name)
	if err != nil {
		return nil, err
	}
	ctx = withSubscriptionCacheOwner(ctx, sub.Name)
	sub, err = normalizeSubscription(sub)
	if err != nil {
		return nil, err
	}
	if sub.Type != domain.SubscriptionTypeRemote {
		return nil, domain.NewError(domain.CodeInvalidArgument, "subscription traffic requires remote subscription")
	}
	remote, err := s.fetchRemoteInput(ctx, *sub.Remote)
	if err != nil {
		return nil, err
	}
	traffic, _ := s.subscriptionTrafficFromRemote(sub, remote)
	result := &domain.SubscriptionTrafficResult{
		SubscriptionName: sub.Name,
		Type:             sub.Type,
		Format:           sub.Format,
		Traffic:          subscriptionTrafficItem(traffic),
	}
	return result.Clone(), nil
}
