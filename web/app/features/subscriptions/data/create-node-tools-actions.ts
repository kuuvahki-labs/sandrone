import { nodeIPInfoFromAPI } from "~/features/subscriptions/model/codec";
import type { NodeURIResult, SubscriptionPreviewNode } from "~/features/subscriptions/model/types";
import type { ApiClient } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";
import { asRecord, stringField } from "~/shared/resources/model-fields";
import { warningsFromAPI } from "~/shared/resources/warning-codec";
import { copyText } from "~/shared/ui/text-transfer";

export function createNodeToolsActions({
  client,
  showNotice,
  t,
}: {
  client: ApiClient;
  showNotice: (message: string, severity?: "success" | "error" | "warning") => void;
  t: Translator;
}) {
  async function renderNodeURI(node: SubscriptionPreviewNode): Promise<NodeURIResult> {
    if (!node.raw) throw new Error(t("subscriptions.nodeTools.missingData"));
    const response = asRecord(await client.inspectNode({ node: node.raw, include: ["uri"] }));
    const uriInfo = asRecord(response.uri);
    const uri = stringField(uriInfo.value).trim();
    if (!uri || uri.includes("\n")) {
      throw new Error(t("subscriptions.nodeTools.renderFailed"));
    }
    return { uri, warnings: warningsFromAPI(uriInfo.warnings) };
  }

  async function copyURI(uri: string): Promise<boolean> {
    if (!(await copyText(uri))) {
      showNotice(t("subscriptions.nodeTools.copyUnavailable"), "warning");
      return false;
    }
    showNotice(t("subscriptions.nodeTools.copied"));
    return true;
  }

  async function lookupNodeIPInfo(node: SubscriptionPreviewNode) {
    if (!node.raw || !node.server) throw new Error(t("subscriptions.nodeTools.ip.missingServer"));
    const response = asRecord(await client.inspectNode({ node: node.raw, include: ["ip"] }));
    return nodeIPInfoFromAPI(response.ip);
  }

  return { copyURI, lookupNodeIPInfo, renderNodeURI };
}
