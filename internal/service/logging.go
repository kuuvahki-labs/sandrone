package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (s *Service) log(ctx context.Context, level slog.Level, msg string, attrs ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Log(ctx, level, msg, attrs...)
}

func elapsedMillis(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func probeCounts(result *domain.ProbeResult) (int, int, int, int) {
	if result == nil {
		return 0, 0, 0, 0
	}
	success := 0
	unsupported := 0
	failure := 0
	cacheHits := 0
	if result.Report.Probe != nil {
		success = result.Report.Probe.SuccessCount
		unsupported = result.Report.Probe.UnsupportedCount
		failure = result.Report.Probe.FailureCount
		cacheHits = result.Report.Probe.CacheHitCount
	}
	if success == 0 && unsupported == 0 && failure == 0 {
		for _, item := range result.Results {
			switch {
			case item.Alive:
				success++
			case item.ErrorCode == string(domain.CodeProbeNodeUnsupported):
				unsupported++
			default:
				failure++
			}
		}
	}
	if cacheHits == 0 {
		for _, item := range result.Results {
			if item.CacheHit {
				cacheHits++
			}
		}
	}
	return success, unsupported, failure, cacheHits
}
