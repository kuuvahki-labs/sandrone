import CloudOutlinedIcon from "@mui/icons-material/CloudOutlined";
import DescriptionOutlinedIcon from "@mui/icons-material/DescriptionOutlined";
import RocketLaunchOutlinedIcon from "@mui/icons-material/RocketLaunchOutlined";

import type { FileDriverIcon as FileDriverIconName } from "~/features/files/drivers/core/file-driver";
import { ClientLogoIcon } from "~/shared/ui/client-logo-icon";

export function FileDriverIcon({ icon }: { icon: FileDriverIconName }) {
  switch (icon) {
    case "mihomo":
      return <ClientLogoIcon client="mihomo" />;
    case "sing-box":
      return <ClientLogoIcon client="sing-box" />;
    case "remote":
      return <CloudOutlinedIcon aria-hidden color="action" />;
    case "rocket":
      return <RocketLaunchOutlinedIcon aria-hidden color="action" />;
    default:
      return <DescriptionOutlinedIcon aria-hidden color="action" />;
  }
}
