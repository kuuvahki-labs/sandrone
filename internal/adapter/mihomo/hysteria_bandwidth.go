package mihomo

import (
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/adapter/shared"
	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func applyMihomoHysteriaRate(node *domain.NodeIR, value, speedValue any, rawKey, speedKey string, text *string, mbps *int) {
	if speedValue != nil {
		speed, err := shared.NormalizeHysteriaMbps(speedValue)
		if err != nil {
			shared.AddRaw(node.Raw, "mihomo."+speedKey, speedValue)
		} else if speed > 0 {
			raw := strings.TrimSpace(shared.StringValue(value))
			if raw != "" {
				if _, err := shared.NormalizeHysteriaRate(raw, shared.HysteriaImplicitMbps); err != nil {
					shared.AddRaw(node.Raw, "mihomo."+rawKey, value)
				}
			}
			*mbps = speed
			return
		}
	}
	raw := strings.TrimSpace(shared.StringValue(value))
	if raw == "" {
		return
	}
	rate, err := shared.NormalizeHysteriaRate(raw, shared.HysteriaImplicitMbps)
	if err != nil {
		shared.AddRaw(node.Raw, "mihomo."+rawKey, value)
		return
	}
	*text, *mbps = rate.Text, rate.Mbps
}
