import { index, route, type RouteConfig } from "@react-router/dev/routes";

export default [
  index("routes/_index.tsx", { id: "home" }),
  route("subscriptions", "routes/subscriptions.tsx", { id: "subscriptions" }),
  route("subscriptions/new", "routes/subscriptions.new.tsx", { id: "subscriptions-new" }),
  route("subscriptions/:kind/:name/edit", "routes/subscriptions.$kind.$name.edit.tsx", { id: "subscriptions-edit" }),
  route("subscriptions/:kind/:name/preview", "routes/subscriptions.$kind.$name.preview.tsx", { id: "subscriptions-preview" }),
  route("files", "routes/files.tsx", { id: "files" }),
  route("files/new", "routes/files.new.tsx", { id: "files-new" }),
  route("files/:name/edit", "routes/files.$name.edit.tsx", { id: "files-edit" }),
  route("files/:name/preview", "routes/files.$name.preview.tsx", { id: "files-preview" }),
  route("shares", "routes/shares.tsx", { id: "shares" }),
  route("settings", "routes/settings.tsx", { id: "settings" }),
  route("settings/service", "routes/settings.service.tsx", { id: "settings-service" }),
  route("settings/data", "routes/settings.data.tsx", { id: "settings-data" }),
] satisfies RouteConfig;
