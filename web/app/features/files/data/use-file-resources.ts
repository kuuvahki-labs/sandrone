import { useCallback, useState } from "react";

import {
  fileDetailFromAPI,
  filePreviewFromAPI,
  filesFromResourceList,
} from "~/features/files/model/codec";
import type { FileDetail, FileItem } from "~/features/files/model/types";
import { type ApiClient, ApiError } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";
import { sortResourceItems } from "~/shared/resources/sort";
import {
  type ResourceErrorNotice,
  type ResourceListState,
  useResourceList,
} from "~/shared/resources/use-resource-list";

export interface FileResourcePorts {
  readonly client: ApiClient;
  readonly showNotice: ResourceErrorNotice;
  readonly t: Translator;
}

export function useFileResources({ client, showNotice, t }: FileResourcePorts): ResourceListState<FileItem> {
  const load = useCallback(() => client.listFiles(), [client]);
  return useResourceList({ load, map: sortedFilesFromResourceList, showNotice, t });
}

export function useFileDetailsResource({ client, showNotice, t }: FileResourcePorts) {
  const [fileDetails, setFileDetails] = useState<Record<string, FileDetail>>({});

  const loadFileDetail = useCallback(async (name: string) => {
    try {
      const detail = fileDetailFromAPI(await client.getFileSpec(name));
      setFileDetails((current) => ({ ...current, [name]: detail }));
      return detail;
    } catch (error) {
      showDetailError(error, showNotice, t, "errors.fileDefinitionLoadFailed");
      return null;
    }
  }, [client, showNotice, t]);

  const loadFilePreview = useCallback(async (name: string, options: { refresh?: boolean } = {}) => {
    try {
      const response = options.refresh
        ? await client.previewFile(name, { refresh: true })
        : await client.previewFile(name);
      return filePreviewFromAPI(response);
    } catch (error) {
      showDetailError(error, showNotice, t, "errors.filePreviewFailed");
      return null;
    }
  }, [client, showNotice, t]);

  return { fileDetails, loadFileDetail, loadFilePreview };
}

function sortedFilesFromResourceList(resourceList: unknown): FileItem[] {
  return sortResourceItems(filesFromResourceList(resourceList));
}

function showDetailError(
  error: unknown,
  showNotice: ResourceErrorNotice,
  t: Translator,
  fallbackKey: Parameters<Translator>[0],
) {
  if (!(error instanceof ApiError && error.status === 401)) {
    showNotice(error instanceof Error ? error.message : t(fallbackKey), "error");
  }
}
