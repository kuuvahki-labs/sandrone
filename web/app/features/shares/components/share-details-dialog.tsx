import { useRef } from "react";
import Button from "@mui/material/Button";
import DialogActions from "@mui/material/DialogActions";
import Typography from "@mui/material/Typography";

import type { CopyShareResult } from "~/features/shares/data/create-share-actions";
import type { ShareItem } from "~/features/shares/model/types";
import { type TranslationKey, useI18n } from "~/shared/i18n/context";
import { AppDialog } from "~/shared/ui/dialogs";
import { QrCodePanel } from "~/shared/ui/qr-code";
import { hasSelectionWithin, selectContents } from "~/shared/ui/text-transfer";

export function ShareDetailsDialog({
  item,
  onClose,
  onCopy,
}: {
  item: ShareItem;
  onClose: () => void;
  onCopy: () => Promise<CopyShareResult>;
}) {
  const { locale, t } = useI18n();
  const publicUrlElement = useRef<HTMLElement | null>(null);

  async function copyLink() {
    if (!(await onCopy()).copied) {
      selectContents(publicUrlElement.current);
    }
  }

  return (
    <AppDialog title={t("shares.details.title")} onClose={onClose}>
      <dl className="grid grid-cols-[minmax(7rem,auto)_minmax(0,1fr)] gap-x-4 gap-y-3">
        <DetailRow label={t("shares.details.name")} value={item.title} />
        <DetailRow label={t("shares.details.id")} value={item.id} mono />
        <DetailRow
          label={t("shares.details.targetKind")}
          value={item.targetKind ? t(`shares.details.targetKind.${item.targetKind}`) : t("shares.details.unknown")}
        />
        <DetailRow label={t("shares.details.targetName")} value={item.targetName || t("shares.details.unknown")} />
        <DetailRow label={t("shares.details.targetFormat")} value={item.targetFormat || t("shares.details.notSet")} />
        <DetailRow label={t("shares.details.status")} value={t(`shares.status.${item.status}`)} />
        <DetailRow label={t("shares.details.validFrom")} value={formatTimestamp(item.validFrom, locale, t)} dateTime={item.validFrom} />
        <DetailRow label={t("shares.details.validUntil")} value={formatTimestamp(item.validUntil, locale, t)} dateTime={item.validUntil} />
        <DetailRow label={t("shares.details.encryption")} value={item.ageRecipient ? "age X25519" : t("shares.details.notEnabled")} />
        {item.createdAt ? (
          <DetailRow label={t("shares.details.createdAt")} value={formatTimestamp(item.createdAt, locale, t)} dateTime={item.createdAt} />
        ) : null}
        {item.updatedAt ? (
          <DetailRow label={t("shares.details.updatedAt")} value={formatTimestamp(item.updatedAt, locale, t)} dateTime={item.updatedAt} />
        ) : null}
        <Typography color="text.secondary" component="dt" variant="body2">
          {t("shares.details.publicUrl")}
        </Typography>
        <dd className="m-0 min-w-0">
          <Typography
            className="block cursor-text break-words select-text"
            component="code"
            ref={publicUrlElement}
            variant="body2"
            onClick={(event) => {
              if (!hasSelectionWithin(event.currentTarget)) {
                selectContents(event.currentTarget);
              }
            }}
          >
            {item.publicUrl}
          </Typography>
        </dd>
      </dl>
      <div className="mt-5 grid gap-3">
        <QrCodePanel label={t("shares.qrcode.label", { name: item.title })} value={item.publicUrl} />
      </div>
      <DialogActions className="px-0 pb-0">
        <Button type="button" onClick={onClose}>{t("actions.close")}</Button>
        <Button type="button" variant="contained" onClick={() => { void copyLink(); }}>
          {t("shares.actions.copy")}
        </Button>
      </DialogActions>
    </AppDialog>
  );
}

function DetailRow({
  dateTime,
  label,
  mono = false,
  value,
}: {
  dateTime?: string;
  label: string;
  mono?: boolean;
  value: string;
}) {
  return (
    <>
      <Typography color="text.secondary" component="dt" variant="body2">{label}</Typography>
      <Typography className={`m-0 min-w-0 break-words${mono ? " font-mono" : ""}`} component="dd" variant="body2">
        {dateTime ? <time dateTime={dateTime}>{value}</time> : value}
      </Typography>
    </>
  );
}

function formatTimestamp(value: string | undefined, locale: string, t: (key: TranslationKey) => string): string {
  if (!value) return t("shares.details.notSet");
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return value;
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "medium" }).format(timestamp);
}
