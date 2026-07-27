import { type ReactNode, useEffect, useState } from "react";
import TextField from "@mui/material/TextField";

import { FileConfigEditor } from "~/features/files/config/components/editor";
import { WorkbenchGroupSection } from "~/features/files/config/components/editor-shared";
import type { LoadSubscriptionPreview } from "~/features/files/config/components/node-source";
import type { LoadRuleSetCatalog } from "~/features/files/config/components/rule-set-catalog-dialog";
import type { ConfigNamingLocale } from "~/features/files/config/model/naming";
import type { FileDriverDefinition } from "~/features/files/drivers/core/file-driver";
import type { FileConfigDetail, FileSourceDetail } from "~/features/files/model/types";
import { FileProcessorBuilder } from "~/features/files/processors/processor-builder";
import { useI18n } from "~/shared/i18n/context";
import type { ProcessorDetail, ResourceOption } from "~/shared/resources/types";
import { RenderCachePolicyField } from "~/shared/ui/render-cache-policy-field";

import { requireFileDriverUI } from "./file-driver-ui-registry";
import { FileTypeSummary } from "./file-type-summary";
import { RawFileConfigEditor } from "./raw-config-editor";
import { FileSourceEditor } from "./source-editor";

export interface FileFormFieldsProps {
  defaultName: string;
  configDefault?: FileConfigDetail;
  description?: string;
  displayName?: string;
  driver: Readonly<FileDriverDefinition>;
  loadSubscriptionPreview?: LoadSubscriptionPreview;
  loadRuleSetCatalog?: LoadRuleSetCatalog;
  mode: "create" | "edit";
  onDirty?: () => void;
  onValidityChange?: (valid: boolean) => void;
  processorsDefault?: ProcessorDetail[];
  renderCacheTTLSeconds?: number;
  scriptFiles?: ResourceOption[];
  sourceDefault?: FileSourceDetail;
  sourceEditorKey?: string;
  subscriptions?: ResourceOption[];
}

export function FileFormFields({ configDefault, defaultName, description = "", displayName = "", driver, loadRuleSetCatalog, loadSubscriptionPreview, mode, onDirty, onValidityChange, processorsDefault, renderCacheTTLSeconds, scriptFiles, sourceDefault, sourceEditorKey, subscriptions = [] }: FileFormFieldsProps) {
  const { locale, t } = useI18n();
  const initialConfigDraft = driver.configuration.mode === "structured"
    ? driver.configuration.adapter.decode(configDefault, locale)
    : undefined;
  const [stableNamingLocale] = useState(() => (
    driver.configuration.mode === "structured"
      ? driver.configuration.adapter.templates.resolveNamingLocale(
        initialConfigDraft
          ? driver.configuration.adapter.toNativeDraft(initialConfigDraft)
          : undefined,
        locale,
      )
      : locale
  ));
  const [configValid, setConfigValid] = useState(!(
    mode === "create"
    && driver.configuration.mode === "structured"
    && driver.configuration.requiresValidOnCreate
  ));
  const [sourceValid, setSourceValid] = useState(true);
  const [processorsValid, setProcessorsValid] = useState(true);
	const isConfig = driver.configuration.mode !== "none";
  const defaultBase = driver.source.defaultBase(stableNamingLocale);
  const processors = mode === "create"
		? driver.processors.defaults()
		: processorsDefault;

  useEffect(() => {
    onValidityChange?.((!isConfig || configValid && sourceValid) && processorsValid);
  }, [configValid, isConfig, onValidityChange, processorsValid, sourceValid]);

  return (
    <div className="grid gap-4">
      <WorkbenchGroupSection collapsible={false} id="file-basic-settings" label={t("files.form.basic")}>
        <div className="grid gap-4 md:grid-cols-2">
          <TextField disabled={mode === "edit"} fullWidth required={mode === "create"} defaultValue={defaultName} label={t("labels.name")} name="name" />
          <TextField fullWidth defaultValue={displayName} label={t("files.form.displayName")} name="display_name" />
          <TextField fullWidth multiline className="md:col-span-2" defaultValue={description} label={t("files.form.description")} minRows={2} name="description" />
          <FileTypeSummary
            icon={driver.presentation.icon}
            label={t(driver.presentation.labelKey)}
            title={t("files.form.fileType")}
          />
          <RenderCachePolicyField defaultValue={renderCacheTTLSeconds} />
        </div>
      </WorkbenchGroupSection>
      {isConfig ? (
        <section aria-label={t("files.form.config")} className="grid min-w-0 gap-3">
          <FileKindConfigWorkbench
            baseEditor={(
              <FileSourceEditor
                contentLabel={t("files.form.content")}
				defaultValue={mode === "create" ? { type: "inline", content: defaultBase } : sourceDefault}
				inlineFallback={defaultBase}
                key={`${sourceEditorKey ?? "file"}-${driver.kind}-base`}
				language={driver.source.syntax}
                onValidityChange={setSourceValid}
                placeholder={driver.source.basePlaceholder}
                preserveImplicit={driver.source.strategy === "optional-base"}
                remoteURLPlaceholder={driver.source.remoteURLPlaceholder}
                validateSource={driver.source.validate}
              />
            )}
            configDefault={configDefault}
            createNamingLocale={stableNamingLocale}
            key={driver.kind}
            loadSubscriptionPreview={loadSubscriptionPreview}
            loadRuleSetCatalog={loadRuleSetCatalog}
            mode={mode}
            driver={driver}
            subscriptions={subscriptions}
            onDirty={onDirty}
            onValidityChange={setConfigValid}
          />
        </section>
      ) : (
        <WorkbenchGroupSection collapsible={false} id="file-content-source" label={t("files.form.contentSource")}>
          <FileSourceEditor defaultValue={sourceDefault} key={sourceEditorKey} />
        </WorkbenchGroupSection>
      )}
      <WorkbenchGroupSection keepMounted defaultExpanded id="file-processors" label={t("files.form.processors")}>
        <FileProcessorBuilder defaultValue={processors} key={driver.kind} kind={driver.kind} onValidityChange={setProcessorsValid} scriptFiles={scriptFiles} />
      </WorkbenchGroupSection>
    </div>
  );
}

export function FileKindConfigWorkbench({
  baseEditor,
  configDefault,
  createNamingLocale,
  loadRuleSetCatalog,
  loadSubscriptionPreview,
  mode,
  onDirty,
  onValidityChange,
  driver,
  subscriptions,
}: {
  baseEditor: ReactNode;
  configDefault?: FileConfigDetail;
  createNamingLocale: ConfigNamingLocale;
  loadSubscriptionPreview?: LoadSubscriptionPreview;
  loadRuleSetCatalog?: LoadRuleSetCatalog;
  mode: "create" | "edit";
  onDirty?: () => void;
  onValidityChange?: (valid: boolean) => void;
  driver: Readonly<FileDriverDefinition>;
  subscriptions: ResourceOption[];
}) {
  if (driver.configuration.mode !== "structured") {
    return (
      <RawFileConfigEditor
        baseEditor={baseEditor}
        configDefault={configDefault}
        onValidityChange={onValidityChange}
        subscriptions={subscriptions}
      />
    );
  }
  return (
    <FileConfigEditor
      adapter={driver.configuration.adapter}
      baseEditor={baseEditor}
      createNamingLocale={createNamingLocale}
      defaultValue={driver.configuration.adapter.decode(configDefault, createNamingLocale)}
      loadSubscriptionPreview={loadSubscriptionPreview}
      loadRuleSetCatalog={loadRuleSetCatalog}
      mode={mode}
      subscriptions={subscriptions}
      ui={requireFileDriverUI(driver.kind)}
      onDirty={onDirty}
      onValidityChange={onValidityChange}
    />
  );
}
