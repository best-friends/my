// SPDX-License-Identifier: AGPL-3.0-only
// Go serves the HTML (ADR-0028), so Vite never transforms an index.html; this sets up React
// Fast Refresh under `my serve -dev-vite-origin` and is empty in the build.
import "@vitejs/plugin-react/preamble";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App, StartupError } from "./App";
import { readRuntimeConfig, type RuntimeConfig } from "./lib/config";
import { conformToContentSecurityPolicy } from "./lib/csp";
import { loadStartI18n } from "./lib/locale";
import { registerServiceWorker, reloadOnPreloadError } from "./lib/pwa";
import { createAppRouter } from "./router";
import { createAppServices } from "./lib/services";
import "./index.css";

// Before any component renders: React Aria must stay inside the shell's CSP.
conformToContentSecurityPolicy(document);

// Before anything loads a chunk: a deploy may have removed the chunks of this version (ADR-0028).
reloadOnPreloadError(window);

const container = document.getElementById("root");
if (container === null) throw new Error("the shell has no #root element");
const root = createRoot(container);

// The page is rendered in the supported locale closest to the one it was last displayed in on this browser, or
// else to the instance's, which the server writes into <html lang> (lib/locale).
const i18n = await loadStartI18n(document);

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
  // The built app only: the Vite dev server builds no worker, and under `my serve -dev-vite-origin` the
  // scripts come from another origin. Without the worker the app works the same, only not offline.
  if (import.meta.env.PROD && "serviceWorker" in navigator) {
    registerServiceWorker(navigator.serviceWorker, () => {
      window.location.reload();
    }).catch((error: unknown) => {
      console.error("the Service Worker could not be registered", error);
    });
  }
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
