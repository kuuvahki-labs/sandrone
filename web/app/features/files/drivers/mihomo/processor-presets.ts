import type { ProcessorDetail } from "~/shared/resources/types";

export type MihomoProcessorPresetID = "sniffer" | "tun" | "tailscale" | "tailnet-share";

const PRESET_MARKER_PREFIX = "# sandrone:mihomo-preset=";

const PRESET_CONTENT: Record<MihomoProcessorPresetID, string> = {
  sniffer: `# sandrone:mihomo-preset=sniffer
sniffer!:
  enable: true
  force-dns-mapping: true
  parse-pure-ip: true
  override-destination: true
  skip-domain:
    - "Mijia Cloud"
    - "dlg.io.mi.com"
    - "+.push.apple.com"
  sniff:
    HTTP:
      ports:
        - 80
        - "8080-8880"
    TLS:
      ports:
        - 443
        - 8443
    QUIC:
      ports:
        - 443
        - 8443`,
  tun: `# sandrone:mihomo-preset=tun
tun!:
  enable: true
  stack: mixed
  auto-route: true
  auto-detect-interface: true
  dns-hijack:
    - any:53
    - tcp://any:53
  route-exclude-address:
    - 10.0.0.0/8
    - 172.16.0.0/12
    - 192.168.0.0/16
    - 169.254.0.0/16
    - fe80::/10
    - fc00::/7`,
  tailscale: `# sandrone:mihomo-preset=tailscale
dns:
  fake-ip-filter+:
    - "+.tailscale.com"
    - "+.ts.net"
  nameserver-policy:
    "<+.ts.net>": 100.100.100.100
tun:
  route-exclude-address+:
    - 100.64.0.0/10
    - fd7a:115c:a1e0::/48`,
  "tailnet-share": `# sandrone:mihomo-preset=tailnet-share
lan-allowed-ips+:
  - 100.64.0.0/10
  - fd7a:115c:a1e0::/48`,
};

const PRESET_NAMES: Record<MihomoProcessorPresetID, string> = {
  sniffer: "Sniffer",
  tun: "TUN",
  tailscale: "Tailscale 共存",
  "tailnet-share": "Tailnet 代理共享",
};

export function mihomoProcessorPreset(id: MihomoProcessorPresetID): ProcessorDetail {
  return {
    name: PRESET_NAMES[id],
    type: "merge",
    stage: "file",
    params: { mode: "yaml_override", content: PRESET_CONTENT[id] },
  };
}

export function defaultMihomoProcessors(): ProcessorDetail[] {
  return [mihomoProcessorPreset("sniffer"), mihomoProcessorPreset("tun")];
}

export function recognizeMihomoProcessorPreset(processor: Pick<ProcessorDetail, "type" | "params">): MihomoProcessorPresetID | null {
  if (processor.type !== "merge" || processor.params?.mode !== "yaml_override" || typeof processor.params.content !== "string") {
    return null;
  }
  for (const id of Object.keys(PRESET_CONTENT) as MihomoProcessorPresetID[]) {
    if (processor.params.content.includes(`${PRESET_MARKER_PREFIX}${id}`)) return id;
  }
  return null;
}
