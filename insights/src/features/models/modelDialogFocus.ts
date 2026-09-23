export function restoreModelDialogFocus(
  event: { preventDefault: () => void },
  opener: HTMLElement | null,
) {
  event.preventDefault();
  const fallback = document.getElementById("model-catalog-search");
  const openerDisabled = opener instanceof HTMLButtonElement && opener.disabled;
  const openerAriaDisabled = opener?.getAttribute("aria-disabled") === "true";
  const target = opener?.isConnected && !openerDisabled && !openerAriaDisabled ? opener : fallback;
  if (target instanceof HTMLElement) target.focus();
}
