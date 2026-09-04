import { Component, lazy, type ReactNode, Suspense } from "react";
import Skeleton from "@mui/material/Skeleton";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

const QRCodeSVG = lazy(async () => {
  const module = await import("qrcode.react");
  return { default: module.QRCodeSVG };
});

export function QrCodePanel({ label, value }: { label: string; value: string }) {
  const { t } = useI18n();
  return (
    <div className="grid justify-items-center gap-2">
      <QrCodeErrorBoundary fallback={<QrCodeFallback />} resetKey={value}>
        <Suspense fallback={<Skeleton aria-label={t("qrcode.loading")} height={256} variant="rounded" width={256} />}>
          <QRCodeSVG
            aria-label={label}
            bgColor="#ffffff"
            boostLevel
            className="h-auto max-w-full rounded bg-white"
            fgColor="#000000"
            level="L"
            marginSize={2}
            role="img"
            size={256}
            value={value}
          />
        </Suspense>
      </QrCodeErrorBoundary>
    </div>
  );
}

function QrCodeFallback() {
  const { t } = useI18n();
  return (
    <Typography color="text.secondary" component="p" role="status" variant="body2">
      {t("qrcode.tooLong")}
    </Typography>
  );
}

class QrCodeErrorBoundary extends Component<{
  children: ReactNode;
  fallback: ReactNode;
  resetKey: string;
}, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidUpdate(previous: Readonly<{ resetKey: string }>) {
    if (this.state.failed && previous.resetKey !== this.props.resetKey) {
      this.setState({ failed: false });
    }
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}
