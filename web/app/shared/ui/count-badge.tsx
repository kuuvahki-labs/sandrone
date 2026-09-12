import Typography from "@mui/material/Typography";

export function CountBadge({ count }: { count: number }) {
  return (
    <Typography
      aria-hidden
      className="shrink-0 rounded-full bg-action-hover px-2 py-0.5"
      color="text.secondary"
      component="span"
      data-slot="count-badge"
      variant="caption"
    >
      {count}
    </Typography>
  );
}
