import type { ProcessorDetail } from "~/shared/resources/types";

const SNIFF_AND_DNS_HIJACK_CONTENT = JSON.stringify({
  route: {
    "+rules": [
      { action: "sniff" },
      {
        type: "logical",
        mode: "or",
        rules: [{ protocol: "dns" }, { port: 53 }],
        action: "hijack-dns",
      },
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
