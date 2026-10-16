// SPDX-License-Identifier: AGPL-3.0-only
// Go serves the HTML (ADR-0028), so Vite never transforms an index.html; this sets up React
// Fast Refresh under `my serve -dev-vite-origin` and is empty in the build.
import "@vitejs/plugin-react/preamble";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { IntlProvider } from "react-intl";
import { App } from "./App";
import { locale, messages } from "./lib/i18n";
import "./index.css";

const container = document.getElementById("root");
if (container === null) throw new Error("index.html has no #root element");

createRoot(container).render(
  <StrictMode>
    <IntlProvider locale={locale} defaultLocale="en" messages={messages}>
      <App />
    </IntlProvider>
  </StrictMode>,
);
