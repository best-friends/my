// SPDX-License-Identifier: AGPL-3.0-only
import { Button, Spinner, TextArea } from "@my/ui";
import { useT } from "../../../lib/i18n";
import { maxDescriptionLength } from "../hooks/policy";
import { type AttachmentView, descriptionFieldId } from "../hooks/useAttachments";
import { useFormatRemaining } from "../hooks/useFormatRemaining";

export type AttachmentListProps = {
  items: readonly AttachmentView[];
  onDescribe: (key: string, description: string) => void;
  /** Removes an image; without it, the images stay (editing a post changes only their descriptions). */
  onRemove?: (key: string) => void;
  /** Whether the user's preferences ask for a description of every image. */
  requireDescriptions: boolean;
  /**
   * Whether the server requires a description of every image (`limits.require_media_description`): the fields are
   * required, and the browser keeps the form from being sent while one is empty or only spaces.
   */
  isDescriptionRequired?: boolean;
  isDisabled?: boolean;
};

/**
 * The images of the draft. Every image has a description field, always shown, whatever its state
 * (web-client.md Y9): the description can be written while the file uploads.
 */
export function AttachmentList({ items, onDescribe, onRemove, requireDescriptions, isDescriptionRequired = false, isDisabled = false }: AttachmentListProps) {
  const t = useT();
  const hint = isDescriptionRequired
    ? "client:compose.media.description.server_required"
    : requireDescriptions
      ? "client:compose.media.description.required"
      : "client:compose.media.description.hint";
  const formatRemaining = useFormatRemaining();
  if (items.length === 0) return null;
  return (
    <ul aria-label={t("client:compose.media.list")} className="flex flex-col gap-3">
      {items.map((item) => (
        <li key={item.key} className="flex flex-col gap-2 rounded-md border border-border p-2 @md:flex-row">
          <div className="relative grid aspect-video w-full shrink-0 place-items-center overflow-hidden rounded-sm bg-surface-sunken @md:w-40">
            {/* The description is the field next to the preview; the preview itself adds nothing for a screen reader. */}
            {item.src !== null && <img src={item.src} alt="" className="size-full object-cover" />}
            {(item.status === "uploading" || item.status === "processing") && (
              <span className="absolute inset-x-0 bottom-0 flex items-center gap-2 bg-surface-raised px-2 py-1 text-sm">
                <Spinner size="sm" label={t(item.status === "uploading" ? "client:compose.media.uploading" : "client:compose.media.processing")} />
                <span aria-hidden="true">{t(item.status === "uploading" ? "client:compose.media.uploading" : "client:compose.media.processing")}</span>
              </span>
            )}
          </div>
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            {item.status === "failed" && <p className="text-sm text-danger">{t("client:compose.media.failed", { reason: item.message })}</p>}
            <TextArea
              id={descriptionFieldId(item.key)}
              label={t("client:compose.media.description.label", { position: String(item.position) })}
              description={t(hint)}
              value={item.description}
              isRequired={isDescriptionRequired}
              validate={(value) => (isDescriptionRequired && value.trim() === "" ? t("client:compose.media.description.missing") : null)}
              onChange={(description) => {
                onDescribe(item.key, description);
              }}
              rows={2}
              characterLimit={maxDescriptionLength}
              formatRemaining={formatRemaining}
              isReadOnly={isDisabled}
            />
            {onRemove !== undefined && (
              <Button
                size="sm"
                variant="ghost"
                className="self-start"
                isDisabled={isDisabled}
                onPress={() => {
                  onRemove(item.key);
                }}
              >
                {t("client:compose.media.remove", { position: String(item.position) })}
              </Button>
            )}
          </div>
        </li>
      ))}
    </ul>
  );
}
