package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

type quickSetting string

const (
	quickSettingDefault  quickSetting = "default"
	quickSettingEnabled  quickSetting = "enabled"
	quickSettingDisabled quickSetting = "disabled"
)

type QuickSettingsParams struct {
	UDP           string `json:"udp,omitempty" jsonschema:"UDP relay override" enum:"default,enabled,disabled" default:"default"`
	TFO           string `json:"tfo,omitempty" jsonschema:"TCP Fast Open override" enum:"default,enabled,disabled" default:"default"`
	AllowInsecure string `json:"allow_insecure,omitempty" jsonschema:"TLS certificate verification override" enum:"default,enabled,disabled" default:"default"`
	VMessAEAD     string `json:"vmess_aead,omitempty" jsonschema:"VMess AEAD override" enum:"default,enabled,disabled" default:"default"`
	Reuse         string `json:"reuse,omitempty" jsonschema:"Snell connection reuse override" enum:"default,enabled,disabled" default:"default"`
}

type quickSettingsProc struct {
	udp           quickSetting
	tfo           quickSetting
	allowInsecure quickSetting
	vmessAEAD     quickSetting
	reuse         quickSetting
}

func buildQuickSettings(spec domain.ProcessorSpec) (domain.NodeProcessor, error) {
	var params QuickSettingsParams
	if err := processor.UnmarshalParams(spec, &params); err != nil {
		return nil, err
	}
	udp, err := parseQuickSetting(spec.Type, "udp", params.UDP)
	if err != nil {
		return nil, err
	}
	tfo, err := parseQuickSetting(spec.Type, "tfo", params.TFO)
	if err != nil {
		return nil, err
	}
	allowInsecure, err := parseQuickSetting(spec.Type, "allow_insecure", params.AllowInsecure)
	if err != nil {
		return nil, err
	}
	vmessAEAD, err := parseQuickSetting(spec.Type, "vmess_aead", params.VMessAEAD)
	if err != nil {
		return nil, err
	}
	reuse, err := parseQuickSetting(spec.Type, "reuse", params.Reuse)
	if err != nil {
		return nil, err
	}
	return &quickSettingsProc{
		udp:           udp,
		tfo:           tfo,
		allowInsecure: allowInsecure,
		vmessAEAD:     vmessAEAD,
		reuse:         reuse,
	}, nil
}

func parseQuickSetting(processorName, field, value string) (quickSetting, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return quickSettingDefault, nil
	}
	switch quickSetting(normalized) {
	case quickSettingDefault, quickSettingEnabled, quickSettingDisabled:
		return quickSetting(normalized), nil
	default:
		return "", &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("quick_settings %s must be one of enabled, disabled, default", field),
			Processor: processorName,
		}
	}
}

func (p *quickSettingsProc) Name() string { return "quick_settings" }

func (p *quickSettingsProc) ApplyNodes(_ context.Context, in domain.NodeProcessInput) (domain.NodeProcessOutput, error) {
	out := make([]domain.NodeIR, len(in.Nodes))
	warnings := []domain.Warning{}
	for i, node := range in.Nodes {
		updated, nodeWarnings := p.apply(node)
		out[i] = updated
		warnings = append(warnings, nodeWarnings...)
	}
	return domain.NodeProcessOutput{Nodes: out, Warnings: warnings}, nil
}

func (p *quickSettingsProc) apply(node domain.NodeIR) (domain.NodeIR, []domain.Warning) {
	updated := node
	warnings := []domain.Warning{}

	switch p.udp {
	case quickSettingEnabled:
		dialer := ensureDialerCopy(&updated)
		dialer.UDPRelay = boolPtr(true)
	case quickSettingDisabled:
		dialer := ensureDialerCopy(&updated)
		dialer.UDPRelay = boolPtr(false)
	}

	switch p.tfo {
	case quickSettingEnabled:
		dialer := ensureDialerCopy(&updated)
		dialer.TFO = true
	case quickSettingDisabled:
		if updated.Dialer != nil {
			dialer := ensureDialerCopy(&updated)
			dialer.TFO = false
		}
	}

	switch p.allowInsecure {
	case quickSettingEnabled:
		tls := ensureTLSCopy(&updated)
		tls.Enabled = true
		tls.InsecureSkipVerify = true
	case quickSettingDisabled:
		if updated.TLS != nil {
			tlsCopy := *updated.TLS
			tlsCopy.InsecureSkipVerify = false
			updated.TLS = &tlsCopy
		}
	}

	if updated.Type == domain.NodeTypeVMess {
		switch p.vmessAEAD {
		case quickSettingEnabled:
			updated.AlterID = 0
		case quickSettingDisabled:
			if updated.AlterID == 0 {
				warnings = append(warnings, domain.Warning{
					Code:    "quick_settings_vmess_aead_legacy_unavailable",
					Message: "vmess_aead=disabled requires an existing non-zero alter_id; leaving VMess alter_id unchanged",
					Node:    updated.Name,
					Field:   "alter_id",
					Source:  "quick_settings",
				})
			}
		}
	}

	if updated.Type == domain.NodeTypeSnell && updated.Snell != nil {
		snell := *updated.Snell
		updated.Snell = &snell
		switch p.reuse {
		case quickSettingEnabled:
			switch snell.Version {
			case 2, 4, 5:
				snell.Reuse = boolPtr(true)
			default:
				warnings = append(warnings, snellReuseWarning(updated, "enabled"))
			}
		case quickSettingDisabled:
			switch snell.Version {
			case 4, 5:
				snell.Reuse = boolPtr(false)
			case 2:
				snell.Reuse = boolPtr(true)
				warnings = append(warnings, snellReuseWarning(updated, "disabled"))
			case 1, 3:
				// These versions do not support reuse, so disabled is already satisfied.
			default:
				warnings = append(warnings, snellReuseWarning(updated, "disabled"))
			}
		}
	}

	return updated, warnings
}

func snellReuseWarning(node domain.NodeIR, value string) domain.Warning {
	return domain.Warning{
		Code:    "quick_settings_snell_reuse_unavailable",
		Message: fmt.Sprintf("reuse=%s is unavailable for Snell v%d; leaving reuse unchanged", value, node.Snell.Version),
		Node:    node.Name,
		Field:   "snell.reuse",
		Source:  "quick_settings",
	}
}

func ensureDialerCopy(node *domain.NodeIR) *domain.DialerOptions {
	if node.Dialer == nil {
		node.Dialer = &domain.DialerOptions{}
		return node.Dialer
	}
	dialerCopy := *node.Dialer
	node.Dialer = &dialerCopy
	return node.Dialer
}

func ensureTLSCopy(node *domain.NodeIR) *domain.TLSOptions {
	if node.TLS == nil {
		node.TLS = &domain.TLSOptions{}
		return node.TLS
	}
	tlsCopy := *node.TLS
	node.TLS = &tlsCopy
	return node.TLS
}

func boolPtr(value bool) *bool {
	return &value
}
