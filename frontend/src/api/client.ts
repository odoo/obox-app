/**
 * HTTP API client — mirrors every Wails binding so the frontend can use the
 * same functionality when running in a remote browser.
 *
 * All privileged calls include auth headers via authState.getAuthHeaders().
 * A 401 response throws an ApiError so callers can clear the session token
 * and re-prompt for the PIN.
 */

import {
  clearPINSessionToken,
  getAuthHeaders,
  setPINSessionToken,
} from "./authState";

// ── Types (mirror the Wails models namespace) ─────────────────────────────────

export interface ApiPrinter {
  name: string;
  ip: string;
  id: string;
  isLAN: boolean;
  lanIp?: string;
  online: boolean;
  type: string;
}

export interface ApiUnavailablePrinter {
  name: string;
  errorMsg: string;
  isLAN: boolean;
  lanIp?: string;
}

export interface ApiPrintersResponse {
  errorMsg: string;
  printers: ApiPrinter[];
  unavailablePrinters: ApiUnavailablePrinter[];
}

export interface ApiWebViewConfig {
  url: string;
  enabled: boolean;
  hasPIN: boolean;
  exitCorners?: string[];
  fullscreen?: boolean;
  reloadCount?: number;
}

export interface ApiAppVariable {
  serverRunning: boolean;
  os: string;
  appId?: string;
  ipAddress?: string;
  networkPrintingEnabled?: boolean;
  autoStart?: boolean;
  fullscreen?: boolean;
  kioskMode?: boolean;
}

export interface ApiTroubleshootInfo {
  activeFirewall: string;
  firewallZone: string;
  port: number;
  subnet: string;
  localIp: string;
  execPath: string;
}

// ── Error type ────────────────────────────────────────────────────────────────

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// ── Internal fetch helpers ────────────────────────────────────────────────────

async function apiFetch(
  path: string,
  options: RequestInit = {},
  privileged = false,
): Promise<Response> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(privileged ? getAuthHeaders() : {}),
    ...(options.headers as Record<string, string> | undefined),
  };

  const res = await fetch(path, { ...options, headers });

  if (res.status === 401 && privileged) {
    clearPINSessionToken();
    throw new ApiError(401, "authentication required");
  }
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, message);
  }
  return res;
}

async function apiGet<T>(path: string): Promise<T> {
  const res = await apiFetch(path);
  return res.json() as Promise<T>;
}

async function apiPost<T>(
  path: string,
  body: unknown,
  privileged = false,
): Promise<T> {
  const res = await apiFetch(
    path,
    { method: "POST", body: JSON.stringify(body) },
    privileged,
  );
  return res.json() as Promise<T>;
}

async function apiDelete<T>(
  path: string,
  body: unknown,
  privileged = false,
): Promise<T> {
  const res = await apiFetch(
    path,
    { method: "DELETE", body: JSON.stringify(body) },
    privileged,
  );
  return res.json() as Promise<T>;
}

// ── Read-only APIs ────────────────────────────────────────────────────────────

export function apiGetAppVariable(): Promise<ApiAppVariable> {
  return apiGet<ApiAppVariable>("/api/app");
}

export function apiGetPrinters(): Promise<ApiPrintersResponse> {
  return apiGet<ApiPrintersResponse>("/api/printers");
}

export function apiGetLANPrinterStatus(ip: string): Promise<{ online: boolean }> {
  return apiGet<{ online: boolean }>(`/api/printers/lan/${encodeURIComponent(ip)}/status`);
}

export function apiGetWebViewConfig(): Promise<ApiWebViewConfig> {
  return apiGet<ApiWebViewConfig>("/api/webview");
}

export function apiGetTroubleshootInfo(): Promise<ApiTroubleshootInfo> {
  return apiGet<ApiTroubleshootInfo>("/api/troubleshoot");
}

// ── PIN session creation ──────────────────────────────────────────────────────

/**
 * Validates pin against the server's stored PIN. On success, stores the
 * returned session token in sessionStorage and returns true.
 */
export async function apiCreatePINSession(pin: string): Promise<boolean> {
  try {
    const res = await apiFetch("/api/auth/session", {
      method: "POST",
      body: JSON.stringify({ pin }),
    });
    const body = (await res.json()) as { token?: string };
    if (body.token) {
      setPINSessionToken(body.token);
      return true;
    }
    return false;
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      return false;
    }
    throw err;
  }
}

// ── Privileged APIs ───────────────────────────────────────────────────────────

export function apiAddLANPrinter(ip: string): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>("/api/printers/lan", { ip }, true);
}

export function apiRemoveLANPrinter(ip: string): Promise<{ ok: boolean }> {
  return apiDelete<{ ok: boolean }>("/api/printers/lan", { ip }, true);
}

export function apiSetWebViewURL(url: string): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>("/api/webview/url", { url }, true);
}

export function apiSetWebViewEnabled(enabled: boolean): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>("/api/webview/enabled", { enabled }, true);
}

export function apiSetWebViewExitCorners(corners: string[]): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>("/api/webview/corners", { corners }, true);
}

export function apiSetWebViewFullscreen(fullscreen: boolean): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>("/api/webview/fullscreen", { fullscreen }, true);
}

export function apiTestPrint(printerId: string): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>(
    `/api/printers/${printerId}/test-print`,
    {},
    true,
  );
}

export function apiCashDrawer(printerId: string): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>(
    `/api/printers/${printerId}/cash-drawer`,
    {},
    true,
  );
}

export function apiReloadKiosk(): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>("/api/webview/reload", {}, true);
}

export function apiQuitApp(): Promise<{ ok: boolean }> {
  return apiPost<{ ok: boolean }>("/api/app/quit", {}, true);
}
