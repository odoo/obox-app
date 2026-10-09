import { useContext } from "react";
import { AppContext } from "../contexts/AppContext";
import { UpdateContext } from "../contexts/UpdateContext";
import Dialog, { type ActionType } from "./Dialog";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";

export default function AboutDialog() {
  const appContext = useContext(AppContext);
  const updateContext = useContext(UpdateContext);
  const { version, buildTime, commit, os, isWindows, isMac, isLinux, serverIsRunning } = appContext.data;

  const updateData = updateContext?.data;
  const updateActions = updateContext?.actions;

  const info = updateData?.info;
  const status = updateData?.status ?? info?.state ?? "checking";
  const error = updateData?.error || info?.error || updateData?.errorMessage || "";
  const progress = updateData?.progress ?? 0;

  const displayVersion = version || "local";
  const formattedVersion = displayVersion.startsWith("v") ? displayVersion : `v${displayVersion}`;

  const osName = isWindows ? "Windows" : isMac ? "macOS" : isLinux ? "Linux" : os || "Unknown";

  const formattedBuildTime = (() => {
    if (!buildTime || buildTime === "unknown") return null;
    try {
      const d = new Date(buildTime);
      if (isNaN(d.getTime())) return buildTime;
      return d.toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return buildTime;
    }
  })();

  const handleUpdate = () => {
    if (updateActions?.handleUpdate) {
      updateActions.handleUpdate();
    }
  };

  const handleCancel = () => {
    if (updateActions?.handleCancel) {
      updateActions.handleCancel();
    }
  };

  const handleCheckUpdates = () => {
    if (updateActions?.checkForUpdate) {
      updateActions.checkForUpdate();
    }
  };

  const manualDownloadUrl = "https://github.com/djip-odoo/obox-app/releases/latest";

  return (
    <Dialog
      title="About Obox App"
      maxWidth="max-w-sm"
      showTitleDivider={true}
      actions={[
        {
          name: "ok",
          label: "OK",
          variant: "primary" as ActionType,
        },
      ]}
      openButton={
        <div className="fixed bottom-3 left-3 sm:bottom-4 sm:left-4 z-30">
          <button
            type="button"
            className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium text-gray-700 bg-white/90 hover:bg-white backdrop-blur-sm border border-gray-200/80 hover:border-odoo/40 shadow-xs hover:shadow-md transition-all duration-200 active:scale-[0.98] cursor-pointer select-none group focus:outline-none focus-visible:ring-2 focus-visible:ring-odoo/30"
            title="View About & System Status"
          >
            <span className="relative flex h-2 w-2 shrink-0">
              {serverIsRunning ? (
                <>
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500 shadow-xs" />
                </>
              ) : (
                <span className="relative inline-flex rounded-full h-2 w-2 bg-rose-500 ring-2 ring-rose-100" />
              )}
            </span>

            <span className="font-mono text-[11px] font-semibold text-gray-700 tracking-tight group-hover:text-odoo transition-colors">
              {formattedVersion}
            </span>

            <svg
              className="w-3.5 h-3.5 text-gray-400 group-hover:text-odoo group-hover:rotate-6 transition-all duration-200 shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth="2"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          </button>
        </div>
      }
    >
      <div className="space-y-4">
        {/* App Branding Header */}
        <div className="flex items-center gap-3.5 pb-0.5">
          <div className="w-12 h-12 rounded-xl bg-odoo/10 text-odoo flex items-center justify-center shrink-0 border border-odoo/20 shadow-2xs">
            <svg
              className="w-6 h-6"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth="2"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M17 17h2a2 2 0 002-2v-4a2 2 0 00-2-2H5a2 2 0 00-2 2v4a2 2 0 002 2h2m2 4h6a2 2 0 002-2v-4a2 2 0 00-2-2H9a2 2 0 00-2 2v4a2 2 0 002 2zm8-12V5a2 2 0 00-2-2H9a2 2 0 00-2 2v4h10z"
              />
            </svg>
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <span className="text-base font-semibold text-gray-900 leading-tight">Obox App</span>
              <span className="font-mono text-[10px] font-semibold text-odoo bg-odoo/10 px-1.5 py-0.5 rounded border border-odoo/20">
                {formattedVersion}
              </span>
            </div>
            <div className="text-xs text-gray-500 mt-0.5">Odoo POS Hardware & Printing Service</div>
          </div>
        </div>

        {/* System Details */}
        <div className="rounded-xl border border-gray-200/80 bg-gray-50/50 p-1 text-xs">
          <div className="divide-y divide-gray-200/60">
            <div className="flex items-center justify-between px-3 py-2">
              <span className="text-gray-500">Service</span>
              <span className="inline-flex items-center gap-1.5 font-medium text-gray-800">
                <span
                  className={`w-1.5 h-1.5 rounded-full ${
                    serverIsRunning ? "bg-emerald-500" : "bg-rose-500"
                  }`}
                />
                {serverIsRunning ? "Active" : "Stopped"}
              </span>
            </div>

            <div className="flex items-center justify-between px-3 py-2">
              <span className="text-gray-500">Version</span>
              <div className="flex items-center gap-1.5 font-mono text-gray-800 font-medium">
                <span>{displayVersion}</span>
                {commit && commit !== "none" && (
                  <span className="text-gray-400 font-normal">({commit.slice(0, 7)})</span>
                )}
              </div>
            </div>

            <div className="flex items-center justify-between px-3 py-2">
              <span className="text-gray-500">Platform</span>
              <span className="font-medium text-gray-800">{osName}</span>
            </div>

            {formattedBuildTime && (
              <div className="flex items-center justify-between px-3 py-2">
                <span className="text-gray-500">Build Date</span>
                <span className="text-gray-600 font-mono text-[11px]">{formattedBuildTime}</span>
              </div>
            )}
          </div>
        </div>


        {/* STATE: checking */}
        {status === "checking" && (
          <div key="checking" className="animate-fade-in bg-gray-50/90 border border-gray-200/80 rounded-xl p-3.5 text-xs flex items-center gap-3 text-gray-700 shadow-2xs">
            <div className="w-8 h-8 rounded-xl bg-odoo/10 text-odoo flex items-center justify-center shrink-0">
              <svg className="w-4 h-4 animate-spin text-odoo" viewBox="0 0 24 24" fill="none">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
              </svg>
            </div>
            <div className="min-w-0">
              <p className="font-semibold text-gray-800 leading-tight">Checking for updates…</p>
              <p className="text-[11px] text-gray-500 mt-0.5">Connecting to GitHub releases server</p>
            </div>
          </div>
        )}

        {/* STATE: downloading */}
        {status === "downloading" && (
          <div key="downloading" className="animate-fade-in bg-gray-50/90 border border-gray-200/80 rounded-xl p-3.5 text-xs space-y-2 shadow-2xs">
            <div className="flex items-center justify-between text-gray-700">
              <div className="flex items-center gap-2">
                <div className="w-4 h-4 border-2 border-odoo border-t-transparent rounded-full animate-spin shrink-0" />
                <span className="font-semibold text-gray-800">Downloading update…</span>
              </div>
              <span className="font-mono font-medium text-gray-600">{progress}%</span>
            </div>
            <div className="h-2 rounded-full bg-gray-200 overflow-hidden">
              <div
                className="h-full bg-odoo transition-all duration-300"
                style={{ width: `${Math.max(progress, 3)}%` }}
              />
            </div>
            <div className="flex items-center justify-between text-[11px] text-gray-500 pt-0.5">
              <span>Please wait while the update is being downloaded.</span>
              <button
                type="button"
                onClick={handleCancel}
                className="text-gray-600 hover:text-red-700 font-medium hover:underline cursor-pointer transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        )}

        {/* STATE: installing */}
        {status === "installing" && (
          <div key="installing" className="animate-fade-in bg-gray-50/90 border border-gray-200/80 rounded-xl p-3.5 text-xs flex items-center gap-3 text-gray-700 shadow-2xs">
            <div className="w-5 h-5 border-2 border-odoo border-t-transparent rounded-full animate-spin shrink-0" />
            <div className="min-w-0">
              <p className="font-semibold text-gray-800 leading-tight">Installing update & restarting…</p>
              <p className="text-[11px] text-gray-500 mt-0.5">The application will automatically restart once finished.</p>
            </div>
          </div>
        )}

        {/* STATE: idle (Up to date) */}
        {status === "idle" && (
          <div key="idle" className="animate-fade-in bg-emerald-50/80 border border-emerald-200/90 rounded-xl p-3.5 text-xs flex items-center justify-between gap-3 text-emerald-900 shadow-2xs">
            <div className="flex items-center gap-2.5 min-w-0">
              <div className="w-8 h-8 rounded-xl bg-emerald-100 text-emerald-700 flex items-center justify-center shrink-0">
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2.5">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                </svg>
              </div>
              <div className="min-w-0">
                <p className="font-semibold text-emerald-900 leading-tight">Already up to date</p>
                <p className="text-[11px] text-emerald-700 mt-0.5">
                  You are running the latest version of Obox App ({displayVersion}).
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={handleCheckUpdates}
              className="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs font-semibold text-emerald-800 bg-white hover:bg-emerald-100/60 border border-emerald-200 rounded-lg shadow-2xs active:scale-95 transition-all duration-150 cursor-pointer shrink-0"
              title="Check again"
            >
              Check Again
            </button>
          </div>
        )}

        {/* STATE: update_available (Newer version available) */}
        {status === "update_available" && info && (
          <div key="update_available" className="animate-fade-in bg-amber-50/80 border border-amber-200/90 rounded-xl p-3.5 text-xs space-y-3 shadow-2xs">
            <div className="flex items-center justify-between gap-2">
              <div className="font-semibold text-gray-800 flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-amber-500 animate-pulse" />
                Update Available
              </div>
              <span className="font-mono font-medium text-amber-900 bg-amber-100 px-2 py-0.5 rounded border border-amber-200/60 shadow-2xs">
                {info.latestVersion}
              </span>
            </div>

            {info.notes && (
              <p className="text-gray-600 line-clamp-3 whitespace-pre-line text-[11px] bg-white/70 p-2.5 rounded-lg border border-amber-100/80">
                {info.notes}
              </p>
            )}

            <div className="space-y-2">
              <button
                type="button"
                onClick={handleUpdate}
                className="w-full inline-flex items-center justify-center gap-2 px-3 py-2 text-xs font-semibold text-white bg-odoo hover:bg-odoo-dark active:opacity-95 rounded-xl shadow-xs transition-all duration-150 cursor-pointer"
              >
                <svg
                  className="w-3.5 h-3.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                  />
                </svg>
                Download & Install ({info.latestVersion})
              </button>

              <div className="flex items-center justify-between text-[11px] text-amber-800 pt-0.5">
                <span>Prefer manual update?</span>
                <button
                  type="button"
                  onClick={() => BrowserOpenURL(manualDownloadUrl)}
                  className="text-odoo font-medium underline inline-flex items-center gap-1 hover:opacity-80 cursor-pointer"
                >
                  <span>Download Manually</span>
                  <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                  </svg>
                </button>
              </div>
            </div>
          </div>
        )}

        {/* STATE: failed (Error during check, download, or install) */}
        {status === "failed" && (
          <div key="failed" className="animate-fade-in bg-red-50/90 border border-red-200/90 rounded-xl p-3.5 text-xs space-y-3 text-red-900 shadow-2xs">
            <div className="flex items-start justify-between gap-3">
              <div className="flex items-start gap-2.5 min-w-0">
                <div className="w-8 h-8 rounded-xl bg-red-100 text-red-700 flex items-center justify-center shrink-0 mt-0.5">
                  <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                </div>
                <div className="min-w-0">
                  <p className="font-semibold text-red-900 leading-tight">Unable to check for updates</p>
                  <p className="text-[11px] text-red-700 mt-1 leading-relaxed">
                    {error || "Cannot reach update server. Please check your internet connection."}
                  </p>
                </div>
              </div>

              <button
                type="button"
                onClick={handleCheckUpdates}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-red-800 bg-white hover:bg-red-100/60 border border-red-200 rounded-lg shadow-2xs active:scale-95 transition-all duration-150 cursor-pointer shrink-0"
              >
                <svg
                  className="w-3.5 h-3.5 text-red-700"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Retry
              </button>
            </div>

            {/* Manual download link on error */}
            <div className="pt-2.5 border-t border-red-200/60 flex items-center justify-between gap-2 text-[11px]">
              <span className="text-red-700">You can download the update manually:</span>
              <button
                type="button"
                onClick={() => BrowserOpenURL(manualDownloadUrl)}
                className="text-odoo font-medium underline inline-flex items-center gap-1 hover:opacity-80 cursor-pointer shrink-0"
              >
                <span>Download Manually</span>
                <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
              </button>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
