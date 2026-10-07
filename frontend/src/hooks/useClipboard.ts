import { useCallback, useContext, useEffect, useRef, useState } from "react";
import { ToastContext } from "../contexts/ToastContext";
import { errorText } from "../error";
import { ClipboardSetText } from "../../wailsjs/runtime/runtime";

async function writeTextToClipboard(text: string): Promise<void> {
  // 1. Wails native runtime clipboard (Linux WebKit2GTK, Windows WebView2, macOS WebKit)
  try {
    const ok = await ClipboardSetText(text);
    if (ok) return;
    // Fall back if the native call reports failure.
  } catch {
    // Fall back to browser clipboard APIs.
  }

  // 2. Browser clipboard API (e.g. running in modern browser dev server)
  try {
    await navigator.clipboard.writeText(text);
    return;
  } catch {
    // Fall back if the browser denies clipboard access.
  }

  // 3. Legacy fallback for restricted webviews
  if (typeof document !== "undefined") {
    const textarea = document.createElement("textarea");
    textarea.value = text;
    textarea.readOnly = true;
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    textarea.style.left = "-9999px";
    textarea.style.top = "-9999px";
    document.body.appendChild(textarea);
    try {
      textarea.focus();
      textarea.select();
      const successful = document.execCommand("copy");
      if (!successful) {
        throw new Error("document.execCommand('copy') returned false");
      }
      return;
    } finally {
      document.body.removeChild(textarea);
    }
  }

  throw new Error("Clipboard copy is not supported in this environment");
}

export function useClipboard(timeout = 2000) {
  const toastContext = useContext(ToastContext);
  const [copiedKeys, setCopiedKeys] = useState<Set<string>>(new Set());
  const timeoutsRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  useEffect(() => {
    const timeouts = timeoutsRef.current;
    return () => {
      timeouts.forEach((t) => clearTimeout(t));
      timeouts.clear();
    };
  }, []);

  const copy = useCallback(
    async (text: string, key: string, label?: string) => {
      try {
        await writeTextToClipboard(text);

        const existing = timeoutsRef.current.get(key);
        if (existing) clearTimeout(existing);

        setCopiedKeys((curr) => new Set(curr).add(key));
        toastContext.actions.showToast(`${label ?? key} copied to clipboard!`, "success");

        const t = setTimeout(() => {
          setCopiedKeys((curr) => {
            if (!curr.has(key)) return curr;
            const next = new Set(curr);
            next.delete(key);
            return next;
          });
          timeoutsRef.current.delete(key);
        }, timeout);
        timeoutsRef.current.set(key, t);

        return true;
      } catch (err) {
        toastContext.actions.showToast(`Copy failed: ${errorText(err, "unknown error")}`, "danger");
        return false;
      }
    },
    [toastContext, timeout],
  );

  return {
    copy,
    isCopied: (key: string) => copiedKeys.has(key),
  };
}
