import { expectTypeOf, it } from "vitest";

import type { Translator } from "~/shared/i18n/context";
import type { ResourceOption } from "~/shared/resources/types";
import { useResourceList } from "~/shared/resources/use-resource-list";

type ComponentProps<Component> = Component extends (props: infer Props) => unknown ? Props : never;

type FileNewPageProps = ComponentProps<typeof import("~/features/files/pages/file-new-page").FileNewPage>;
type FileEditPageProps = ComponentProps<typeof import("~/features/files/pages/file-edit-page").FileEditPage>;
type FileFormFieldsProps = ComponentProps<typeof import("~/features/files/editor/file-form").FileFormFields>;
type RawFileConfigEditorProps = ComponentProps<typeof import("~/features/files/editor/raw-config-editor").RawFileConfigEditor>;
type FileProcessorBuilderProps = ComponentProps<typeof import("~/features/files/processors/processor-builder").FileProcessorBuilder>;
type FileConfigNodeSourceProps = ComponentProps<typeof import("~/features/files/config/components/node-source").ConfigNodeSourceSection>;
type SubscriptionNewPageProps = ComponentProps<typeof import("~/features/subscriptions/pages/subscription-new-page").SubscriptionNewPage>;
type SubscriptionEditPageProps = ComponentProps<typeof import("~/features/subscriptions/pages/subscription-edit-page").SubscriptionEditPage>;
type SubscriptionFormFieldsProps = ComponentProps<typeof import("~/features/subscriptions/components/subscription-form").SubscriptionFormFields>;
type SubscriptionProcessorBuilderProps = ComponentProps<typeof import("~/features/subscriptions/components/processor-builder").ProcessorBuilder>;
type ScriptProcessorParamsEditorProps = ComponentProps<typeof import("~/shared/processors/components/script-processor-params-editor").ScriptProcessorParamsEditor>;
type ResourceListOptions = Parameters<typeof useResourceList>[0];

it("uses neutral resource options at cross-feature list ports", () => {
  expectTypeOf<FileNewPageProps["subscriptions"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<FileNewPageProps["scriptFiles"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<FileEditPageProps["subscriptions"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<FileEditPageProps["scriptFiles"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<FileFormFieldsProps["subscriptions"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<FileFormFieldsProps["scriptFiles"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<RawFileConfigEditorProps["subscriptions"]>().toEqualTypeOf<ResourceOption[]>();
  expectTypeOf<FileProcessorBuilderProps["scriptFiles"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<FileConfigNodeSourceProps["subscriptions"]>().toEqualTypeOf<ResourceOption[]>();
  expectTypeOf<SubscriptionNewPageProps["scriptFiles"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<SubscriptionEditPageProps["scriptFiles"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<SubscriptionFormFieldsProps["scriptFiles"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<SubscriptionProcessorBuilderProps["scriptFiles"]>().toEqualTypeOf<ResourceOption[] | undefined>();
  expectTypeOf<ScriptProcessorParamsEditorProps["scriptFiles"]>().toEqualTypeOf<ResourceOption[] | undefined>();
});

it("requires an injected translator at the shared resource boundary", () => {
  expectTypeOf<ResourceListOptions["t"]>().toEqualTypeOf<Translator>();
});
