// SPDX-License-Identifier: AGPL-3.0-only
// A block with the box props of the layout primitives but no layout of its own: a size container, a padded row whose
// content flows as text, a framed block, a capped width. Children one above another with a gap are a `Stack`.
import { createElement } from "react";
import { box, type BoxProps as BoxOwnProps, type LayoutElement, type PolymorphicProps, splitBox } from "./layout";

export type BoxProps<E extends LayoutElement = "div"> = PolymorphicProps<E, BoxOwnProps>;

/** A plain block (`div`, or `as`) with typed padding, border, width and container props. */
export function Box<E extends LayoutElement = "div">(props: BoxProps<E>) {
  const [boxProps, { as, ...rest }] = splitBox(props);
  return createElement(as ?? "div", { ...rest, className: box(boxProps) });
}
