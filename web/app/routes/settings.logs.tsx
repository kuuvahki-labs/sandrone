import { useNavigate } from "react-router";

import { useSandrone } from "~/core/provider/context";
import { SettingsLogsPage } from "~/features/settings/pages/settings-logs-page";

export default function SettingsLogsRoute() {
  const app = useSandrone();
  const navigate = useNavigate();
  return <SettingsLogsPage client={app.client} onBack={() => navigate("/settings")} />;
}
