import { useContext } from "react";
import { getPINSessionToken } from "../api/authState";
import { PINContext } from "../contexts/PINContext";
import { AppContext } from "../contexts/AppContext";

/**
 * usePINGate — returns a `gate` function that wraps any async privileged
 * action with PIN enforcement for remote clients.
 *
 * Behaviour:
 *   Wails context  → executes `action` immediately (trusted, no PIN).
 *   Remote context → checks sessionStorage for an existing PIN session token;
 *                    if absent, shows the PIN dialog first; then calls action.
 *
 * The session token is stored in sessionStorage by apiCreatePINSession()
 * (called from WebViewContext.validatePIN) and persists until tab/page refresh.
 *
 * Usage:
 *   const gate = usePINGate();
 *   await gate(async () => { await somePrivilegedAction(); });
 */
export function usePINGate() {
  const { data: { isWails } } = useContext(AppContext);
  const { showPINDialog } = useContext(PINContext);

  async function gate<T>(action: () => Promise<T>): Promise<T | null> {
    // Wails: trusted — bypass PIN entirely
    if (isWails) {
      return action();
    }

    // Remote: check sessionStorage for an existing session
    if (getPINSessionToken()) {
      try {
        return await action();
      } catch (err: unknown) {
        // 401: session expired / server restarted — re-prompt
        const code =
          (err as { status?: number; code?: number })?.status ??
          (err as { status?: number; code?: number })?.code;
        if (code === 401) {
          // authState.clearPINSessionToken() is called by apiFetch on 401
          const ok = await showPINDialog();
          if (!ok) return null;
          return action();
        }
        throw err;
      }
    }

    // No session — show PIN dialog first
    const ok = await showPINDialog();
    if (!ok) return null;

    return action();
  }

  return gate;
}
