import type { TranslationKey } from "~/shared/i18n/context";

import type { AdaptiveGroupAnchorProblem } from "./adaptive-groups";

export interface AdaptiveGenerationAvailability {
	anchorProblem: AdaptiveGroupAnchorProblem | null;
	editorMode: "wizard" | "advanced";
  hasCurrentPreview: boolean;
  nodeCount: number;
	previewStatus: "idle" | "loading" | "ready" | "error";
	selected: boolean;
}

export function adaptiveGenerationDisabledReasonKey(
  input: AdaptiveGenerationAvailability,
): TranslationKey | undefined {
  if (input.editorMode === "advanced") return "files.config.adaptiveAdvancedUnsupported";
  if (!input.selected) return "files.config.adaptiveSelectSubscription";
  if (input.previewStatus === "loading") return "files.config.adaptivePreviewLoading";
  if (input.previewStatus === "error" || !input.hasCurrentPreview) {
    return "files.config.adaptivePreviewUnavailable";
  }
  if (input.nodeCount === 0) return "files.config.adaptiveNoNodes";

	switch (input.anchorProblem?.code) {
    case "anchor_missing":
      return "files.config.adaptiveProxyMissing";
    case "anchor_duplicate":
      return "files.config.adaptiveProxyDuplicate";
    case "anchor_type_invalid":
      return "files.config.adaptiveProxyTypeInvalid";
    case "anchor_members_invalid":
      return "files.config.adaptiveProxyMembersInvalid";
    default:
      return undefined;
  }
}
