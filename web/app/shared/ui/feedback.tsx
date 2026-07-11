import Alert from "@mui/material/Alert";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Typography from "@mui/material/Typography";

export function EmptyState({ title }: { title: string }) {
  return (
    <Card component="article" variant="outlined">
      <CardContent>
        <Typography component="h2" variant="h6">
          {title}
        </Typography>
      </CardContent>
    </Card>
  );
}

type SnackbarNotice = {
  id: number;
  message: string;
  severity: "success" | "error" | "warning";
};

export function SnackbarStack({ notices }: { notices: SnackbarNotice[] }) {
  if (!notices.length) return null;
  return (
    <div className="pointer-events-none fixed inset-x-0 top-4 z-[1400] grid justify-items-center gap-2 px-4">
      {notices.map((notice) => (
        <Alert className="pointer-events-auto w-full max-w-md shadow-6" key={notice.id} role={notice.severity === "error" ? "alert" : "status"} severity={notice.severity} variant="filled">
          {notice.message}
        </Alert>
      ))}
    </div>
  );
}

export function ResourceLoadingCard({ title }: { title: string }) {
  return (
    <section className="grid gap-6">
      <Card component="article" variant="outlined">
        <CardContent>
          <div className="grid gap-2">
            <Typography component="h2" variant="h5">
              {title}
            </Typography>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}
