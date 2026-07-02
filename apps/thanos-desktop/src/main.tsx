import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createRoot } from "react-dom/client";
import { WorkbenchRoute } from "./routes/WorkbenchRoute";
import "./styles.css";

const app = document.querySelector<HTMLElement>("#app");

if (!app) {
  throw new Error("app root missing");
}

const queryClient = new QueryClient();

createRoot(app).render(
  <QueryClientProvider client={queryClient}>
    <WorkbenchRoute />
  </QueryClientProvider>,
);
