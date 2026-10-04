import { createElement, isValidElement, type ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { AdmissionRejectionNotice } from "./admission-rejection";

const limitRejection = { code: "limit_not_allowed" };

const render = (props: Partial<Parameters<typeof AdmissionRejectionNotice>[0]> = {}) =>
  renderToStaticMarkup(
    createElement(AdmissionRejectionNotice, {
      rejection: limitRejection,
      onResubmit: () => {},
      ...props,
    }),
  );

describe("AdmissionRejectionNotice", () => {
  it("shows the reason code, the safe message and the scope or limit to change", () => {
    const html = render();
    expect(html).toContain("limit_not_allowed");
    expect(html).toContain("A requested limit is above what the active policy allows.");
    expect(html).toContain("Limits (model calls, timeout)");
    expect(html).toContain("A requested limit is higher than the active policy permits.");
  });

  it("says no passport or run exists and that nothing was narrowed for the operator", () => {
    const html = render();
    expect(html).toContain("no passport was issued and no run started");
    expect(html).toContain("Nothing was narrowed for you");
    expect(html).toContain("submit the revised request yourself");
  });

  it("never resubmits by itself: rendering does not call onResubmit and the button is not a form submit", () => {
    const onResubmit = vi.fn();
    const html = render({ onResubmit });
    expect(onResubmit).not.toHaveBeenCalled();
    expect(html).toContain('type="button"');
    expect(html).not.toContain('type="submit"');
  });

  it("calls onResubmit only through the explicit button's click handler", () => {
    const onResubmit = vi.fn();
    const element = AdmissionRejectionNotice({
      rejection: limitRejection,
      onResubmit,
    }) as ReactElement<{ children: unknown[] }>;
    const buttons: ReactElement<{ onClick?: () => void }>[] = [];
    const collect = (node: unknown): void => {
      if (Array.isArray(node)) return node.forEach(collect);
      if (!isValidElement(node)) return;
      const props = node.props as { onClick?: () => void; children?: unknown };
      if (props.onClick) buttons.push(node as ReactElement<{ onClick?: () => void }>);
      collect(props.children);
    };
    collect(element);
    expect(buttons).toHaveLength(1);
    buttons[0]?.props.onClick?.();
    expect(onResubmit).toHaveBeenCalledTimes(1);
  });

  it("disables the button while a submission is in flight", () => {
    expect(render({ isSubmitting: true })).toContain('disabled=""');
    expect(render({ isSubmitting: false })).not.toContain('disabled=""');
  });

  it("shows only the fixed safe message, never upstream wording a caller passes along", () => {
    // The type has no message field; a reply's text must not reach the page even if it is smuggled in.
    const smuggled = { code: "template_not_allowed", message: "<b>upstream</b> detail" };
    const html = render({ rejection: smuggled });
    expect(html).toContain("The requested template is not permitted for this operation.");
    expect(html).not.toContain("upstream");
    expect(html).not.toContain("&lt;b&gt;");
  });

  it("renders nothing for a code admission does not return, instead of a generic banner", () => {
    expect(render({ rejection: { code: "approval_required" } })).toBe("");
  });
});
