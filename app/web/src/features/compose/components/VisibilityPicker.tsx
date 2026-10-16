// SPDX-License-Identifier: AGPL-3.0-only
import { Button, Popover, PopoverTrigger, Radio, RadioGroup } from "@my/ui";
import { useT } from "../../../lib/i18n";
import type { Visibility } from "../api/compose";
import { visibilities } from "../hooks/policy";
import { visibilityDescription, visibilityLabel } from "../hooks/visibilityMessages";

export type VisibilityPickerProps = { value: Visibility; onChange: (value: Visibility) => void; isDisabled?: boolean };

function isVisibility(value: string): value is Visibility {
  return (visibilities as readonly string[]).includes(value);
}

/**
 * The visibility of the post: a button that names the current choice and opens a popover with the five
 * visibilities as a radio group, each with who can see it (ADR-0017; WAI-ARIA APG radio group and dialog
 * patterns). Choosing does not close the popover, because the arrow keys choose as they move.
 */
export function VisibilityPicker({ value, onChange, isDisabled = false }: VisibilityPickerProps) {
  const t = useT();
  const title = t("client:compose.visibility.title");
  return (
    <PopoverTrigger>
      <Button size="sm" isDisabled={isDisabled}>
        {t("client:compose.visibility.button", { visibility: t(visibilityLabel[value]) })}
      </Button>
      <Popover aria-label={title} placement="bottom start">
        {({ close }) => (
          <>
            <RadioGroup
              label={title}
              value={value}
              onChange={(next) => {
                if (isVisibility(next)) onChange(next);
              }}
            >
              {visibilities.map((visibility) => (
                <Radio key={visibility} value={visibility} description={t(visibilityDescription[visibility])}>
                  {t(visibilityLabel[visibility])}
                </Radio>
              ))}
            </RadioGroup>
            <Button size="sm" variant="primary" className="self-end" onPress={close}>
              {t("client:compose.visibility.done")}
            </Button>
          </>
        )}
      </Popover>
    </PopoverTrigger>
  );
}
