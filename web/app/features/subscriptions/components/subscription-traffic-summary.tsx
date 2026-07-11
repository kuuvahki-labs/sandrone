import LinearProgress from "@mui/material/LinearProgress";
import Typography from "@mui/material/Typography";

import type { SubscriptionTraffic, SubscriptionTrafficItem } from "~/features/subscriptions/model/types";

export function SubscriptionTrafficSummary({ traffic }: { traffic: SubscriptionTraffic }) {
  return (
    <div className="grid gap-3">
      <SubscriptionTrafficUsage traffic={traffic} />
    </div>
  );
}

function SubscriptionTrafficUsage({ traffic }: { traffic: SubscriptionTraffic }) {
  const item = traffic.traffic;
  if (!item) {
    return null;
  }
  return (
    <div className="grid gap-3">
      <div className="grid gap-2">
        <SubscriptionTrafficCard item={item} />
      </div>
    </div>
  );
}

function SubscriptionTrafficCard({ item }: { item: SubscriptionTrafficItem }) {
  const summaryText = trafficSummaryText(item);
  const progress = item.totalBytes && item.totalBytes !== 0 ? clampPercent((item.usedBytes / item.totalBytes) * 100) : undefined;
  return (
    <div className="grid min-w-0 gap-2">
      {item.planName ? (
        <Typography className="break-words" component="h6" variant="subtitle2">
          {item.planName}
        </Typography>
      ) : null}
      <Typography color="text.secondary" variant="body2">{summaryText}</Typography>
      {progress !== undefined ? <LinearProgress aria-label={summaryText} value={progress} variant="determinate" /> : null}
    </div>
  );
}

function trafficSummaryText(item: SubscriptionTrafficItem): string {
  const parts = [
    `↑ ${formatBytes(item.uploadBytes)}`,
    `↓ ${formatBytes(item.downloadBytes)}`,
  ];
  if (item.totalBytes !== undefined) {
    parts.push(`TOT ${formatBytes(item.totalBytes)}`);
  }
  if (item.remainingDays !== undefined) {
    parts.push(formatRemainingDays(item.remainingDays));
  }
  return parts.join(" · ");
}

function formatRemainingDays(days: number): string {
  const years = Math.floor(days / 365);
  const remainingDays = days % 365;
  if (years <= 0) {
    return `${days}D`;
  }
  return remainingDays > 0 ? `${years}Y ${remainingDays}D` : `${years}Y`;
}

function formatBytes(value: number): string {
  const sign = value < 0 ? "-" : "";
  let current = Math.abs(value);
  const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
  let unitIndex = 0;
  while (current >= 1024 && unitIndex < units.length - 1) {
    current /= 1024;
    unitIndex += 1;
  }
  const formatted = Number.isInteger(current) ? String(current) : current.toFixed(2).replace(/\.0+$/, "").replace(/(\.\d*[1-9])0+$/, "$1");
  return `${sign}${formatted} ${units[unitIndex]}`;
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.min(100, Math.max(0, value));
}
