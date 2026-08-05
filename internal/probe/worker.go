package probe

import (
	"errors"
	"time"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

const probeWorkerPanicMessage = "probe worker panicked"

func recoverProbeWorkerPanic(result *domain.NodeProbeResult, req domain.ProbeRequest, node domain.NodeIR, code string) {
	if recover() == nil {
		return
	}
	*result = resultForError(req, node, code, errors.New(probeWorkerPanicMessage), time.Now())
}
