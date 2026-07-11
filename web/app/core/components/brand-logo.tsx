import Box from "@mui/material/Box";

const brandLogoAlt = "Sandrone logo";

interface BrandLogoProps {
  size: number;
  src: string;
}

export function BrandLogo({ size, src }: BrandLogoProps) {
  return (
    <Box
      alt={brandLogoAlt}
      component="img"
      height={size}
      src={src}
      sx={{ display: "block", flexShrink: 0, objectFit: "contain" }}
      width={size}
    />
  );
}
