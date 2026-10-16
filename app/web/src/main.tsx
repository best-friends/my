// SPDX-License-Identifier: AGPL-3.0-only
// Go serves the HTML (ADR-0028), so Vite never transforms an index.html; this sets up React
// Fast Refresh under `my serve -dev-vite-origin` and is empty in the build.
import "@vitejs/plugin-react/preamble";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App, StartupError } from "./App";
import { readRuntimeConfig, type RuntimeConfig } from "./lib/config";
import { applyDocumentLocale, loadI18n } from "./lib/i18n";
import { createAppRouter } from "./router";
import { createAppServices } from "./lib/services";
import "./index.css";

const container = document.getElementById("root");
if (container === null) throw new Error("the shell has no #root element");
const root = createRoot(container);

// The server writes the locale into <html lang>; the page is rendered in the closest supported one.
const i18n = await loadI18n(document.documentElement.lang);
applyDocumentLocale(document, i18n.locale);

let config: RuntimeConfig | undefined;
try {
  config = readRuntimeConfig(document);
} catch (error) {
  console.error(error);
}

if (config === undefined) {
  root.render(
    <StrictMode>
      <StartupError i18n={i18n} />
    </StrictMode>,
  );
} else {
  const services = createAppServices(config, i18n);
  // Resume the session from the refresh token cookie while the first page loads; API requests wait for it.
  void services.session.restore();
  const router = createAppRouter(services);
  root.render(
    <StrictMode>
      <App config={config} {...services} router={router} />
    </StrictMode>,
  );
}
