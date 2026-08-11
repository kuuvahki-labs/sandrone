import { useCallback } from "react";

import {
  subscriptionDefinitionFromAPI,
  subscriptionPreviewFromAPI,
  subscriptionsFromResourceList,
  subscriptionTrafficFromAPI,
} from "~/features/subscriptions/model/codec";
import type { SubscriptionItem } from "~/features/subscriptions/model/types";
import { type ApiClient, ApiError } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";
import { sortResourceItems } from "~/shared/resources/sort";
import {
  type ResourceErrorNotice,
  type ResourceListState,
  useResourceList,
} from "~/shared/resources/use-resource-list";

export interface SubscriptionResourcePorts {
  readonly client: ApiClient;
  readonly showNotice: ResourceErrorNotice;
  readonly t: Translator;
}

export function useSubscriptionResources({
  client,
  showNotice,
  t,
}: SubscriptionResourcePorts): ResourceListState<SubscriptionItem> {
  const load = useCallback(() => client.listSubscriptions(), [client]);
  return useResourceList({ load, map: sortedSubscriptionsFromResourceList, showNotice, t });
}

export function useSubscriptionDetailsResource({ client, showNotice, t }: SubscriptionResourcePorts) {
  const loadSubscriptionDefinition = useCallback(async (name: string) => {
    try {
      return subscriptionDefinitionFromAPI(await client.getSubscription(name));
    } catch (error) {
      showDetailError(error, showNotice, t, "errors.subscriptionDefinitionLoadFailed");
      return null;
    }
  }, [client, showNotice, t]);

  const loadSubscriptionPreview = useCallback(async (name: string, options: { refresh?: boolean } = {}) => {
    try {
      const response = options.refresh
        ? await client.previewSubscription(name, { refresh: true })
        : await client.previewSubscription(name);
      return subscriptionPreviewFromAPI(response);
    } catch (error) {
      showDetailError(error, showNotice, t, "errors.subscriptionPreviewFailed");
      return null;
    }
  }, [client, showNotice, t]);

  const loadSubscriptionTraffic = useCallback(async (
    name: string,
    options: { refresh?: boolean } = {},
  ) => {
    try {
      return subscriptionTrafficFromAPI(await client.subscriptionTraffic(name, options));
    } catch (error) {
      showDetailError(
        error,
        showNotice,
        t,
        "errors.requestFailed",
        (reason) => t("errors.subscriptionTrafficFailed", { name, reason }),
      );
      return null;
    }
  }, [client, showNotice, t]);

  return { loadSubscriptionDefinition, loadSubscriptionPreview, loadSubscriptionTraffic };
}

function sortedSubscriptionsFromResourceList(resourceList: unknown): SubscriptionItem[] {
  return sortResourceItems(subscriptionsFromResourceList(resourceList));
}

function showDetailError(
  error: unknown,
  showNotice: ResourceErrorNotice,
  t: Translator,
  fallbackKey: Parameters<Translator>[0],
  formatMessage: (message: string) => string = (message) => message,
) {
  if (!(error instanceof ApiError && error.status === 401)) {
    const message = error instanceof Error ? error.message : t(fallbackKey);
    showNotice(formatMessage(message), "error");
  }
}
