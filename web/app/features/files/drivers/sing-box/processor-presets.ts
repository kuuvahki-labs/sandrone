import type { ProcessorDetail } from "~/shared/resources/types";

const SNIFF_AND_DNS_HIJACK_CONTENT = JSON.stringify({
  route: {
    "+rules": [
      { action: "sniff" },
      { protocol: "dns", action: "hijack-dns" },
    ],
  },
}, null, 2);

export function defaultSingBoxProcessors(): ProcessorDetail[] {
  return [
    {
      name: "Sniff & DNS Hijack",
      type: "merge",
      stage: "file",
      params: {
        mode: "json_override",
        content: SNIFF_AND_DNS_HIJACK_CONTENT,
      },
    },
  ];
}
