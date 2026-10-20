// SPDX-License-Identifier: AGPL-3.0-only
// Where the new-post form lives (the issue's "sheet on mobile, side panel on desktop"). The app shell
// decides which by the viewport (web-client.md D2) and passes it to ComposeProvider; ComposeEntry then
// renders the panel, or the sheet with the button that opens it (the shell shows that button only on pages
// with a feed). Only one form is ever on the page.
import { Button, PlusIcon, Sheet, Tooltip, TooltipTrigger } from "@my/ui";
import { type ReactNode, useId } from "react";
import { useT } from "../../../lib/i18n";
import { type ComposeLayout, ComposeContext, type ComposeController, useComposeContext, useComposeController } from "../hooks/useCompose";
import { useComposeFormPending } from "../hooks/useComposeForm";
import { ComposeForm } from "./ComposeForm";

export type ComposeProviderProps = { layout: ComposeLayout; children: ReactNode };

/** Shares the draft of the signed-in viewer with the form and with the reply buttons of other features. */
export function ComposeProvider({ layout, children }: ComposeProviderProps) {
  const controller = useComposeController(layout);
  return <ComposeContext value={controller}>{children}</ComposeContext>;
}

/**
 * How the sheet's button shows: with its label; as an icon whose accessible name and tooltip are the label
 * (narrow screens); or not at all (pages without a feed, where replies still open the sheet).
 */
type ComposeTrigger = "labelled" | "icon" | "none";

export type ComposeEntryProps = {
  /** The sheet's button; the side panel ignores it. */
  trigger?: ComposeTrigger;
};

/** The form's place in the layout: the side panel, or the sheet and the button that opens it. Nothing while signed out. */
export function ComposeEntry({ trigger = "labelled" }: ComposeEntryProps) {
  const controller = useComposeContext();
  if (controller === null) return null;
  return controller.layout === "panel" ? <ComposePanel controller={controller} /> : <ComposeSheet controller={controller} trigger={trigger} />;
}

function ComposePanel({ controller }: { controller: ComposeController }) {
  const t = useT();
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("client:compose.title")}
      </h2>
      <ComposeForm controller={controller} labelledBy={headingId} />
    </section>
  );
}

/**
 * A button (the shell floats it above the tab bar, on pages with a feed) that opens the form in a sheet (a
 * modal dialog: WAI-ARIA APG dialog pattern). Closing the sheet keeps the draft.
 *
 * The button loads what the form needs, and the sheet opens once it has arrived (or failed), so the form is
 * ready when it mounts and its text area takes focus at once. A sheet that opened on a spinner would focus
 * the dialog itself first, and React Aria then focuses the dialog again 500 ms later if focus is on the body
 * (a workaround for VoiceOver on iOS). That could take focus from a popover in the form that had just closed
 * and was about to return focus to its trigger.
 */
function ComposeSheet({ controller, trigger }: { controller: ComposeController; trigger: ComposeTrigger }) {
  const t = useT();
  const headingId = useId();
  const isFormPending = useComposeFormPending();
  const open = () => {
    controller.setSheetOpen(true);
  };
  return (
    <>
      {trigger === "labelled" && (
        <Button variant="primary" onPress={open} className="min-h-12 rounded-full px-4 shadow-lg">
          <PlusIcon />
          {t("client:compose.open")}
        </Button>
      )}
      {trigger === "icon" && (
        // An icon-only button is named by aria-label and shows the same words as a tooltip (web-client.md G12).
        <TooltipTrigger>
          <Button variant="primary" aria-label={t("client:compose.open")} onPress={open} className="size-12 rounded-full px-0 shadow-lg">
            <PlusIcon />
          </Button>
          <Tooltip placement="start">{t("client:compose.open")}</Tooltip>
        </TooltipTrigger>
      )}
      <Sheet
        title={<span id={headingId}>{t("client:compose.title")}</span>}
        closeLabel={t("client:compose.close")}
        isOpen={controller.isSheetOpen && !isFormPending}
        onOpenChange={controller.setSheetOpen}
      >
        <ComposeForm controller={controller} labelledBy={headingId} autoFocus />
      </Sheet>
    </>
  );
}
