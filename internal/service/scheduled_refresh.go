package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// RunScheduledRefresh runs the internal refresh scheduler until ctx is
// cancelled. It is intended for long-lived serve entrypoints only.
func (s *Service) RunScheduledRefresh(ctx context.Context) {
	if !s.schedulerEnabled {
		s.setScheduledRefreshConfiguration(false, nil)
		<-ctx.Done()
		return
	}
	logger := scheduledRefreshCronLogger{service: s}
	runner := cron.New(
		cron.WithLocation(time.Local),
		cron.WithChain(cron.Recover(logger)),
	)
	var entryID cron.EntryID
	configure := func() {
		if entryID != 0 {
			runner.Remove(entryID)
			entryID = 0
		}
		settings := s.currentSettings().ScheduledRefresh
		if !settings.Enabled {
			s.setScheduledRefreshConfiguration(false, nil)
			return
		}
		schedule, err := cron.ParseStandard(settings.Schedule)
		if err != nil {
			s.setScheduledRefreshConfiguration(false, nil)
			s.log(ctx, slog.LevelError, "scheduled refresh configuration rejected", "error", err)
			return
		}
		entryID, err = runner.AddFunc(settings.Schedule, func() {
			s.runScheduledRefresh(ctx, settings.Targets, schedule)
		})
		if err != nil {
			s.setScheduledRefreshConfiguration(false, nil)
			s.log(ctx, slog.LevelError, "scheduled refresh configuration rejected", "error", err)
			return
		}
		next := schedule.Next(s.now())
		s.setScheduledRefreshConfiguration(true, &next)
	}

	configure()
	runner.Start()
	for {
		select {
		case <-ctx.Done():
			stopCtx := runner.Stop()
			<-stopCtx.Done()
			return
		case <-s.scheduledRefreshUpdates:
			configure()
		}
	}
}

func (s *Service) ScheduledRefreshStatus(context.Context) domain.ScheduledRefreshStatus {
	s.scheduledRefreshMu.Lock()
	defer s.scheduledRefreshMu.Unlock()
	return s.scheduledRefreshStatus
}

func (s *Service) notifyScheduledRefreshSettingsChanged() {
	select {
	case s.scheduledRefreshUpdates <- struct{}{}:
	default:
	}
}

func (s *Service) setScheduledRefreshConfiguration(enabled bool, next *time.Time) {
	s.scheduledRefreshMu.Lock()
	defer s.scheduledRefreshMu.Unlock()
	s.scheduledRefreshStatus.Enabled = enabled
	s.scheduledRefreshStatus.NextRunAt = cloneTime(next)
}

func (s *Service) runScheduledRefresh(ctx context.Context, targets []domain.ScheduledRefreshTarget, schedule cron.Schedule) {
	started := s.now()
	s.scheduledRefreshMu.Lock()
	if s.scheduledRefreshStatus.Running {
		s.scheduledRefreshStatus.SkippedCount++
		s.scheduledRefreshStatus.LastSkippedAt = cloneTime(&started)
		if schedule != nil {
			next := schedule.Next(started)
			s.scheduledRefreshStatus.NextRunAt = cloneTime(&next)
		}
		s.scheduledRefreshMu.Unlock()
		s.log(ctx, slog.LevelWarn, "scheduled refresh skipped because the previous run is still active")
		return
	}
	s.scheduledRefreshStatus.Running = true
	s.scheduledRefreshStatus.LastStartedAt = cloneTime(&started)
	if schedule != nil {
		next := schedule.Next(started)
		s.scheduledRefreshStatus.NextRunAt = cloneTime(&next)
	}
	s.scheduledRefreshMu.Unlock()

	successCount := 0
	failureCount := 0
	defer func() {
		completed := s.now()
		s.scheduledRefreshMu.Lock()
		s.scheduledRefreshStatus.Running = false
		s.scheduledRefreshStatus.LastCompletedAt = cloneTime(&completed)
		s.scheduledRefreshStatus.LastSuccessCount = successCount
		s.scheduledRefreshStatus.LastFailureCount = failureCount
		s.scheduledRefreshMu.Unlock()
		s.log(ctx, slog.LevelInfo, "scheduled refresh completed", "success_count", successCount, "failure_count", failureCount)
	}()

	for _, target := range targets {
		if ctx.Err() != nil {
			return
		}
		err := s.scheduledRefreshTarget(ctx, target)
		if err != nil {
			failureCount++
			s.log(ctx, slog.LevelError, "scheduled refresh target failed",
				"target_kind", target.Kind,
				"target_name", target.Name,
				"error_code", scheduledRefreshErrorCode(err),
				"error", err,
			)
			continue
		}
		successCount++
		s.log(ctx, slog.LevelInfo, "scheduled refresh target completed", "target_kind", target.Kind, "target_name", target.Name)
	}
}

func (s *Service) refreshScheduledTarget(ctx context.Context, target domain.ScheduledRefreshTarget) error {
	switch target.Kind {
	case "subscription":
		_, err := s.PreviewSubscriptionRequest(ctx, domain.SubscriptionPreviewRequest{
			Name:    target.Name,
			Refresh: true,
		})
		return err
	case "file":
		_, err := s.GetFile(ctx, domain.FileRequest{Name: target.Name, Refresh: true})
		return err
	default:
		return domain.NewError(domain.CodeInvalidArgument, "unsupported scheduled refresh target kind")
	}
}

func scheduledRefreshErrorCode(err error) string {
	if appErr, ok := errors.AsType[*domain.AppError](err); ok {
		return string(appErr.Code)
	}
	return "scheduled_refresh_failed"
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

type scheduledRefreshCronLogger struct {
	service *Service
}

func (l scheduledRefreshCronLogger) Info(string, ...interface{}) {}

func (l scheduledRefreshCronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	attrs := make([]any, 0, len(keysAndValues)+2)
	attrs = append(attrs, "error", err)
	attrs = append(attrs, keysAndValues...)
	l.service.log(context.Background(), slog.LevelError, msg, attrs...)
}
