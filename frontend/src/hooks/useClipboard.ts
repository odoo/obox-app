import { useCallback, useContext, useEffect, useRef, useState } from "react";
import { ToastContext } from "../contexts/ToastContext";
import { errorText } from "../error";

export function useClipboard(timeout = 2000) {
  const toastContext = useContext(ToastContext);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  const copy = useCallback(
    async (text: string, key: string, label?: string) => {
      try {
        await navigator.clipboard.writeText(text);

        if (timeoutRef.current) {
          clearTimeout(timeoutRef.current);
        }

        setCopiedKey(key);
        toastContext.actions.showToast(`${label ?? key} copied to clipboard!`, "success",);

        timeoutRef.current = setTimeout(() => { setCopiedKey((curr) => (curr === key ? null : curr)); }, timeout);
        return true;
      } catch (err) {
        toastContext.actions.showToast(`Copy failed: ${errorText(err, "unknown error")}`, "danger",);
        return false;
      }
    },
    [toastContext, timeout],
  );

  return {
    copiedKey,
    copy,
    isCopied: (key: string) => copiedKey === key,
  };
}
