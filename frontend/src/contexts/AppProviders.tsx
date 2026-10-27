import { ReactNode, useEffect } from "react";
import { ToastContextWrapper } from "./ToastContext";
import { AppContextWrapper } from "./AppContext";
import { UpdateContextWrapper } from "./UpdateContext";
import { PrinterContextWrapper } from "./PrinterContext";
import { OdooContextWrapper } from "./OdooContext";
import { onEvent } from "../lib/wails";
import { ReportCrash } from "@bindings/obox-app/app";

// ── Crash reporter ──────────────────────────────────────────────────────────
// Catches unhandled JS errors and promise rejections, then reports them to the
// backend so they appear in logs and the user gets a native dialog.

function useCrashReporter() {
  useEffect(() => {
    const onError = (event: ErrorEvent) => {
      ReportCrash(event.message, event.error?.stack ?? "").catch(() => {});
    };
    const onUnhandledRejection = (event: PromiseRejectionEvent) => {
      const msg = event.reason instanceof Error ? event.reason.message : String(event.reason);
      const stack = event.reason instanceof Error ? (event.reason.stack ?? "") : "";
      ReportCrash(msg, stack).catch(() => {});
    };

    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onUnhandledRejection);
    return () => {
      window.removeEventListener("error", onError);
      window.removeEventListener("unhandledrejection", onUnhandledRejection);
    };
  }, []);
}

// ── Tray event listener ─────────────────────────────────────────────────────
// The backend emits "tray:check-for-updates" when the user clicks the tray
// menu item. We forward it to the update panel by dispatching a custom DOM
// event that UpdateBanner / UpdateContext can listen to.

function useTrayEvents() {
  useEffect(() => {
    const unsub = onEvent("tray:check-for-updates", () => {
      window.dispatchEvent(new CustomEvent("obox:check-for-updates"));
    });
    return () => { if (unsub) unsub(); };
  }, []);
}

// ── Provider tree ───────────────────────────────────────────────────────────

function AppSideEffects() {
  useCrashReporter();
  useTrayEvents();
  return null;
}

export default function AppProviders({ children }: { children: ReactNode }) {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <UpdateContextWrapper>
          <PrinterContextWrapper>
            <OdooContextWrapper>
              <AppSideEffects />
              {children}
            </OdooContextWrapper>
          </PrinterContextWrapper>
        </UpdateContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}
