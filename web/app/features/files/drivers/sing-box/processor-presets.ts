import { isRecord } from "~/features/files/config/model/editor-model";
import { configAnchorName, type ConfigNamingLocale } from "~/features/files/config/model/naming";
import type {
  FileProcessorPreset,
  FileProcessorPresetCategory,
} from "~/features/files/drivers/core/processor-presets";
import legacySingBoxOutboundAdapterScript from "~/features/files/drivers/sing-box/legacy/sing-box-outbound-adapter-reject-empty.js?raw";
import legacySingBoxTailscaleExternalScript from "~/features/files/drivers/sing-box/legacy/sing-box-tailscale-external.js?raw";
import legacySingBoxTailscaleNativeScript from "~/features/files/drivers/sing-box/legacy/sing-box-tailscale-native.js?raw";
import { githubRuleSourceMirrorPreset } from "~/features/files/processors/github-rule-source-mirror-preset";
import {
  orderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "~/features/files/processors/ordered-rule-preset";
import singBoxFakeIPCompatScript from "~/features/files/processors/scripts/sing-box-fakeip-compat.js?raw";
import singBoxOutboundAdapterScript from "~/features/files/processors/scripts/sing-box-outbound-adapter.js?raw";
import singBoxTailnetShareScript from "~/features/files/processors/scripts/sing-box-tailnet-share.js?raw";
import singBoxTailscaleExternalScript from "~/features/files/processors/scripts/sing-box-tailscale-external.js?raw";
import singBoxTailscaleNativeScript from "~/features/files/processors/scripts/sing-box-tailscale-native.js?raw";
import singBoxTunScript from "~/features/files/processors/scripts/sing-box-tun.js?raw";
import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

export type SingBoxProcessorPresetID =
  | "outbound-adapter"
  | "sniff"
  | "quic-fallback"
  | "tun"
  | "tailscale-native"
  | "tailscale-external"
  | "tailnet-share"
  | "fakeip-compat";
type SingBoxOrderedRuleProcessorPresetID = "quic-fallback";
type SingBoxScriptProcessorPresetID = "tun" | "tailscale-native" | "tailscale-external" | "tailnet-share" | "fakeip-compat";

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

const MANAGED_SCRIPTS: Readonly<Record<SingBoxScriptProcessorPresetID, string>> = {
  tun: singBoxTunScript,
  "tailscale-native": singBoxTailscaleNativeScript,
  "tailscale-external": singBoxTailscaleExternalScript,
  "tailnet-share": singBoxTailnetShareScript,
  "fakeip-compat": singBoxFakeIPCompatScript,
};

const FAKEIP_COMPAT_DOMAIN = [
  "time-ios.apple.com",
  "ntp.ntsc.ac.cn",
  "mesu.apple.com",
  "swscan.apple.com",
  "swquery.apple.com",
  "swdownload.apple.com",
  "swcdn.apple.com",
  "swdist.apple.com",
  "music.163.com",
  "y.qq.com",
  "streamoc.music.tc.qq.com",
  "mobileoc.music.tc.qq.com",
  "isure.stream.qqmusic.qq.com",
  "dl.stream.qqmusic.qq.com",
  "aqqmusic.tc.qq.com",
  "amobile.music.tc.qq.com",
  "songsearch.kugou.com",
  "trackercdn.kugou.com",
  "music.migu.cn",
  "ps.res.netease.com",
];

const FAKEIP_COMPAT_DOMAIN_SUFFIX = [
  "cmbchina.com",
  "cmbimg.com",
  "sandai.net",
  "n0808.com",
  "uu.163.com",
  "oray.com",
  "orayimg.com",
];

const FAKEIP_COMPAT_DOMAIN_REGEX = [
  "^time\\.[^.]+\\.gov$",
  "^time\\.[^.]+\\.edu\\.cn$",
  "^time\\.[^.]+\\.apple\\.com$",
  ...Array.from({ length: 7 }, (_, index) => `^time${index + 1}\\.[^.]+\\.com$`),
  ...Array.from({ length: 7 }, (_, index) => `^ntp${index + 1}\\.[^.]+\\.com$`),
  "^[^.]+\\.time\\.edu\\.cn$",
  "^[^.]+\\.ntp\\.org\\.cn$",
  "^[^.]+\\.music\\.163\\.com$",
  "^[^.]+\\.y\\.qq\\.com$",
  "^[^.]+\\.kuwo\\.cn$",
  "^[^.]+\\.music\\.migu\\.cn$",
  "^[^.]+\\.mcdn\\.bilivideo\\.cn$",
];

const ORDERED_RULE_PRESETS: Record<
  SingBoxOrderedRuleProcessorPresetID,
  OrderedRuleProcessorPresetOptions
> = {
  "quic-fallback": {
    id: "quic-fallback",
    kind: "sing-box",
    rules: [{ protocol: "quic", action: "reject" }],
  },
};

export function singBoxProcessorPreset(id: SingBoxProcessorPresetID, name: string): ProcessorDetail {
  if (id === "outbound-adapter") return outboundAdapterProcessor(name);
  if (id === "sniff") return sniffAndDNSHijackProcessor(name);
  if (isOrderedRulePresetID(id)) return orderedRuleProcessorPreset(ORDERED_RULE_PRESETS[id], name);
  return managedScriptProcessor(id, name);
}

export const singBoxProcessorPresets: readonly FileProcessorPreset[] = [
  {
    id: "outbound-adapter",
    configurationUse: {
      matches: (settings) => isRecord(settings) && Array.isArray(settings.groups)
        && settings.groups.some((group) => isRecord(group) && (Object.hasOwn(group, "filter") || Object.hasOwn(group, "exclude-filter"))),
      missingNoticeKey: "files.config.outboundAdapterProcessorMissing",
    },
    category: "network",
    labelKey: "processors.filePreset.singBox.outboundAdapter.label",
    defaultOn: true,
    dependencies: [],
    conflicts: [],
    build: (t) => outboundAdapterProcessor(t("processors.filePreset.singBox.outboundAdapter.label")),
    recognize: (processor) => {
      if (processor.type !== "script" || !isRecord(processor.params)) return false;
      const source = processor.params.source;
      return isExactRecord(source, ["content", "type"])
        && source.type === "inline"
        && (source.content === singBoxOutboundAdapterScript
          || source.content === legacySingBoxOutboundAdapterScript);
    },
    isCurrent: (processor) => processor.type === "script"
      && isRecord(processor.params)
      && inlineSourceContent(processor.params.source) === singBoxOutboundAdapterScript,
    refresh: (processor) => ({
      ...processor,
      enabled: true,
      type: "script",
      stage: "file",
      params: {
        ...processor.params,
        source: { type: "inline", content: singBoxOutboundAdapterScript },
      },
    }),
    configurationNotices: (processor, settings) => {
      const args = processor.params?.args;
      const target = isRecord(args) ? args.default_outbound : undefined;
      if (typeof target !== "string" || !target.trim() || !isRecord(settings)) return [];
      const groups = Array.isArray(settings.groups) ? settings.groups : [];
      if (groups.some((group) => isRecord(group) && group.tag === target)) return [];
      return [{ messageKey: "files.config.defaultOutboundReferenceMissing", params: { target } }];
    },
  },
  {
    id: "sniff",
    category: "network",
    labelKey: "processors.filePreset.singBox.sniff.label",
    defaultOn: true,
    dependencies: [],
    conflicts: [],
    build: (t) => singBoxProcessorPreset("sniff", t("processors.filePreset.singBox.sniff.label")),
    recognize: (processor) => (
      processor.type === "merge"
      && processor.params?.mode === "json_override"
      && processor.params.content === SNIFF_AND_DNS_HIJACK_CONTENT
    ),
  },
  githubRuleSourceMirrorPreset,
  orderedRuleDescriptor(
    ORDERED_RULE_PRESETS["quic-fallback"],
    "network",
    "processors.filePreset.singBox.quicFallback.label",
    ["sniff"],
  ),
  managedScriptDescriptor("tun", "network", "processors.filePreset.singBox.tun.label"),
  managedScriptDescriptor("tailscale-native", "tailscale", "processors.filePreset.singBox.tailscaleNative.label", ["tun"], ["tailscale-external"]),
  managedScriptDescriptor("tailscale-external", "tailscale", "processors.filePreset.singBox.tailscaleExternal.label", ["tun"], ["tailscale-native"]),
  managedScriptDescriptor("tailnet-share", "tailscale", "processors.filePreset.singBox.tailnetShare.label", ["tun", "tailscale-external"]),
  managedScriptDescriptor("fakeip-compat", "network", "processors.filePreset.singBox.fakeIPCompat.label"),
];

export function defaultSingBoxProcessors(t: Translator, { namingLocale }: { namingLocale: ConfigNamingLocale }): ProcessorDetail[] {
  return singBoxProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => {
      const processor = preset.build(t);
      return preset.id === "outbound-adapter"
        ? { ...processor, params: { ...processor.params, args: { default_outbound: configAnchorName(namingLocale) } } }
        : processor;
    });
}

function sniffAndDNSHijackProcessor(name: string): ProcessorDetail {
  return {
    name,
    type: "merge",
    stage: "file",
    params: {
      mode: "json_override",
      content: SNIFF_AND_DNS_HIJACK_CONTENT,
    },
  };
}

function outboundAdapterProcessor(name: string): ProcessorDetail {
  return {
    name,
    type: "script",
    stage: "file",
    params: { source: { type: "inline", content: singBoxOutboundAdapterScript } },
  };
}

function orderedRuleDescriptor(
  options: OrderedRuleProcessorPresetOptions,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  dependencies: readonly SingBoxProcessorPresetID[] = [],
  conflicts: readonly SingBoxProcessorPresetID[] = [],
): FileProcessorPreset {
  return {
    id: options.id,
    category,
    labelKey,
    defaultOn: false,
    dependencies,
    conflicts,
    build: (t) => orderedRuleProcessorPreset(options, t(labelKey)),
    recognize: (processor) => recognizeOrderedRuleProcessorPreset(processor, options),
  };
}

function managedScriptProcessor(id: SingBoxScriptProcessorPresetID, name: string): ProcessorDetail {
  return {
    name,
    type: "script",
    stage: "file",
    params: {
      source: {
        type: "inline",
        content: MANAGED_SCRIPTS[id],
      },
      args: managedScriptDefaultArgs(id),
    },
  };
}

function managedScriptDescriptor(
  id: SingBoxScriptProcessorPresetID,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  dependencies: readonly SingBoxProcessorPresetID[] = [],
  conflicts: readonly SingBoxProcessorPresetID[] = [],
): FileProcessorPreset {
  return {
    id,
    category,
    labelKey,
    defaultOn: false,
    dependencies,
    conflicts,
    build: (t) => managedScriptProcessor(id, t(labelKey)),
    recognize: (processor) => recognizeManagedScriptProcessor(id, processor),
    isCurrent: (processor) => isCurrentManagedScriptProcessor(id, processor),
  };
}

function managedScriptDefaultArgs(id: SingBoxScriptProcessorPresetID): Record<string, unknown> {
  switch (id) {
    case "tailscale-native": return { preset_id: id, auth_key: "" };
    case "tailnet-share": return {
      preset_id: id,
      listen_addresses: [],
      listen_port: 2080,
      username: "",
      password: "",
    };
    case "fakeip-compat": return {
      preset_id: id,
      domain: [...FAKEIP_COMPAT_DOMAIN],
      domain_suffix: [...FAKEIP_COMPAT_DOMAIN_SUFFIX],
      domain_regex: [...FAKEIP_COMPAT_DOMAIN_REGEX],
      server: "",
    };
    default: return { preset_id: id };
  }
}

function recognizeManagedScriptProcessor(
  id: SingBoxScriptProcessorPresetID,
  processor: Pick<ProcessorDetail, "type" | "params">,
): boolean {
  if (processor.type !== "script" || !isRecord(processor.params)) return false;
  const source = inlineSourceContent(processor.params.source);
  if (source === null) return false;
  if (source === MANAGED_SCRIPTS[id]) return validManagedScriptArgs(id, processor.params.args);
  return isLegacyTailscaleSource(id, source) && validLegacyTailscaleArgs(id, processor.params.args);
}

function isCurrentManagedScriptProcessor(
  id: SingBoxScriptProcessorPresetID,
  processor: Pick<ProcessorDetail, "type" | "params">,
): boolean {
  return processor.type === "script"
    && isRecord(processor.params)
    && inlineSourceContent(processor.params.source) === MANAGED_SCRIPTS[id]
    && isRecord(processor.params.args)
    && processor.params.args.preset_id === id;
}

function validManagedScriptArgs(id: SingBoxScriptProcessorPresetID, value: unknown): boolean {
  if (!isRecord(value) || value.preset_id !== id) return false;
  switch (id) {
    case "tailscale-native": return isExactRecord(value, ["auth_key", "preset_id"]) && typeof value.auth_key === "string";
    case "tailnet-share": return isExactRecord(value, ["listen_addresses", "listen_port", "password", "preset_id", "username"])
      && isStringArray(value.listen_addresses)
      && typeof value.listen_port === "number"
      && typeof value.username === "string"
      && typeof value.password === "string";
    case "fakeip-compat": return isExactRecord(value, ["domain", "domain_regex", "domain_suffix", "preset_id", "server"])
      && isStringArray(value.domain)
      && isStringArray(value.domain_suffix)
      && isStringArray(value.domain_regex)
      && typeof value.server === "string";
    default: return isExactRecord(value, ["preset_id"]);
  }
}

function validLegacyTailscaleArgs(id: SingBoxScriptProcessorPresetID, value: unknown): boolean {
  if (id === "tailscale-external") return value === undefined || isExactRecord(value, []);
  return id === "tailscale-native"
    && isExactRecord(value, ["auth_key"])
    && typeof value.auth_key === "string";
}

function inlineSourceContent(value: unknown): string | null {
  return isExactRecord(value, ["content", "type"])
    && value.type === "inline"
    && typeof value.content === "string"
    ? value.content
    : null;
}

function isLegacyTailscaleSource(id: SingBoxScriptProcessorPresetID, source: string): boolean {
  return id === "tailscale-native"
    ? source === legacySingBoxTailscaleNativeScript
    : id === "tailscale-external" && source === legacySingBoxTailscaleExternalScript;
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
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

function isOrderedRulePresetID(
  id: SingBoxProcessorPresetID,
): id is SingBoxOrderedRuleProcessorPresetID {
  return id === "quic-fallback";
}
