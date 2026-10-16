// SPDX-License-Identifier: AGPL-3.0-only
import { Center } from "@my/ui";
import { FormattedMessage } from "react-intl";

/** Placeholder page until the app shell and its routes exist (#126). */
export function App() {
  return (
    <main>
      <Center>
        <h1 className="text-2xl font-semibold">
          <FormattedMessage id="client:app.placeholder" />
        </h1>
      </Center>
    </main>
  );
}
