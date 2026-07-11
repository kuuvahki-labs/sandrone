import { screen, within } from "@testing-library/react";
import type userEvent from "@testing-library/user-event";

import type { ResourceOption } from "~/shared/resources/types";
import type { CreateSpeedDialAction } from "~/shared/ui/resource-list";

import type { ConfigNodePreviewInput } from "./config/model/node-source";
import type { FileItem } from "./model/types";

export const files: FileItem[] = [
  { name: "default.yaml", title: "default.yaml", kind: "static", description: "main", sourceType: "remote", sourceSummary: "远程", processorCount: 1 },
];

export const subscriptionOptions: ResourceOption[] = [
  { name: "provider", title: "provider" },
];

export const noop = () => undefined;

export function createAction(label: string, onSelect: () => void, ariaLabel?: string): CreateSpeedDialAction {
  return { ariaLabel, icon: <span aria-hidden>{label.slice(0, 1)}</span>, label, onSelect };
}

export function configNodePreview(subscriptionName: string, names: string[]): ConfigNodePreviewInput {
  return {
    subscriptionName,
    nodes: names.map((name, index) => ({
      identity: `sha256:${index}`,
      after: { name, type: "ss", endpoint: `node-${index}.example:8388` },
    })),
    warnings: [],
  };
}

type TestUser = ReturnType<typeof userEvent.setup>;

export async function selectMuiOption(user: TestUser, combobox: HTMLElement, optionName: string) {
  await user.click(combobox);
  const listbox = await screen.findByRole("listbox");
  await user.click(within(listbox).getByRole("option", { name: optionName }));
}
