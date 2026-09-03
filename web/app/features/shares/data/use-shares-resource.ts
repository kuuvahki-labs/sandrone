import { useCallback } from "react";

import { sharesFromShareList } from "~/features/shares/model/codec";
import type { ShareItem } from "~/features/shares/model/types";
import type { ApiClient } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";
import { sortResourceItems } from "~/shared/resources/sort";
import {
  type ResourceErrorNotice,
  type ResourceListState,
  useResourceList,
} from "~/shared/resources/use-resource-list";

export interface ShareResourcePorts {
  readonly client: ApiClient;
  readonly publicBaseUrl: string;
  readonly showNotice: ResourceErrorNotice;
  readonly t: Translator;
}

export function useSharesResource({
  client,
  publicBaseUrl,
  showNotice,
  t,
}: ShareResourcePorts): ResourceListState<ShareItem> {
  const load = useCallback(() => client.listShares(), [client]);
  const map = useCallback(
    (resourceList: unknown) => sortSharesFromResourceList(resourceList, publicBaseUrl),
    [publicBaseUrl],
  );
  return useResourceList({ load, map, showNotice, t });
}

function sortSharesFromResourceList(resourceList: unknown, publicBaseUrl: string): ShareItem[] {
  return sortResourceItems(
    sharesFromShareList(resourceList, publicBaseUrl).map((item) => ({
      createdAt: item.createdAt,
      item,
      name: item.id,
      title: item.title,
      updatedAt: item.updatedAt,
    })),
  ).map(({ item }) => item);
}
