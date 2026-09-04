export async function copyText(value: string): Promise<boolean> {
  if (!navigator.clipboard) return false;
  try {
    await navigator.clipboard.writeText(value);
    return true;
  } catch {
    return false;
  }
}

export function selectContents(element: Node | null) {
  if (!element) return;
  const selection = window.getSelection();
  if (!selection) return;
  const range = document.createRange();
  range.selectNodeContents(element);
  selection.removeAllRanges();
  selection.addRange(range);
}

export function hasSelectionWithin(element: Node): boolean {
  const selection = window.getSelection();
  if (!selection || selection.isCollapsed) return false;
  return element.contains(selection.anchorNode) || element.contains(selection.focusNode);
}
