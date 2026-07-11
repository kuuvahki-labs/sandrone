import Box from "@mui/material/Box";

export type ClientLogo = "mihomo" | "sing-box";

const logoSource: Record<ClientLogo, string> = {
  mihomo: "/brand/clients/mihomo.webp",
  "sing-box": "/brand/clients/sing-box.svg",
};

export function ClientLogoIcon({ client, size = 24 }: { client: ClientLogo; size?: number }) {
  return (
    <Box
      alt=""
      aria-hidden
      component="img"
      height={size}
      src={logoSource[client]}
      sx={{ display: "block", flexShrink: 0, objectFit: "contain" }}
      width={size}
    />
  );
}
