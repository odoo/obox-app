/**
 * authState — module-level singleton that holds current auth credentials.
 *
 * - Desktop Wails context: Sets the Wails token once on startup.
 * - Remote Webview context: Stores the PIN session token strictly in-memory.
 *   Upon every page refresh, in-memory state resets to null, requiring re-authentication.
 */

const SESSION_KEY = "epos-pin-session-token";

let _wailsToken: string | null = null;
let _pinSessionToken: string | null = null;

// Clear any stale browser storage on script load
if (typeof window !== "undefined") {
  sessionStorage.removeItem(SESSION_KEY);
}

/** Called once by AppContext when the Wails token is obtained. */
export function setWailsToken(token: string): void {
  _wailsToken = token;
}

/** Returns true when running inside the trusted Wails application. */
export function isWailsContext(): boolean {
  return _wailsToken !== null;
}

/** Retrieves the active in-memory PIN session token. */
export function getPINSessionToken(): string | null {
  return _pinSessionToken;
}

/** Sets a PIN session token for the current page session. */
export function setPINSessionToken(token: string): void {
  _pinSessionToken = token;
}

/** Clears the PIN session token (called on 401 responses or logout). */
export function clearPINSessionToken(): void {
  _pinSessionToken = null;
  if (typeof window !== "undefined") {
    sessionStorage.removeItem(SESSION_KEY);
  }
}

/**
 * Returns the HTTP headers needed to authenticate a privileged request:
 *   - Wails context  → X-Wails-Token
 *   - Remote context → Authorization: Bearer <pin-session-token>
 */
export function getAuthHeaders(): Record<string, string> {
  if (_wailsToken) {
    return { "X-Wails-Token": _wailsToken };
  }
  const session = getPINSessionToken();
  if (session) {
    return { Authorization: `Bearer ${session}` };
  }
  return {};
}
