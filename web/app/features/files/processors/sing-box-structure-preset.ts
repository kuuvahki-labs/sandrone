import type { ProcessorDetail } from "~/shared/resources/types";

import updateSingBoxTunScript from "./scripts/update-sing-box-tun.js?raw";

export type SingBoxStructureOperation =
  | "ensure-tun"
  | "ipv4-only"
  | "udp-p2p-eim"
  | "linux-tun-acceleration"
  | "mptcp-direct"
  | "windows-relaxed-route";

export interface SingBoxStructureProcessorPresetOptions {
  readonly operation: SingBoxStructureOperation;
}

export function singBoxStructureProcessorPreset(
  options: SingBoxStructureProcessorPresetOptions,
  name: string,
): ProcessorDetail {
  return {
    name,
    type: "script",
    stage: "file",
    params: expectedParams(options),
  };
}

export function recognizeSingBoxStructureProcessorPreset(
  processor: Pick<ProcessorDetail, "type" | "params">,
  options: SingBoxStructureProcessorPresetOptions,
): boolean {
  if (processor.type !== "script") return false;
  const params = processor.params;
  if (!isExactRecord(params, ["args", "source"])) return false;
  const source = params.source;
  const args = params.args;
  if (!isExactRecord(source, ["content", "type"])) return false;
  if (!isExactRecord(args, ["operation"])) return false;
  return source.type === "inline"
    && source.content === updateSingBoxTunScript
    && args.operation === options.operation;
}

function expectedParams(
  options: SingBoxStructureProcessorPresetOptions,
): Record<string, unknown> {
  return {
    source: {
      type: "inline",
      content: updateSingBoxTunScript,
    },
    args: { operation: options.operation },
  };
}

function isExactRecord(
  value: unknown,
  expectedKeys: readonly string[],
): value is Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
  const keys = Object.keys(value).sort();
  return keys.length === expectedKeys.length
    && keys.every((key, index) => key === expectedKeys[index]);
}
