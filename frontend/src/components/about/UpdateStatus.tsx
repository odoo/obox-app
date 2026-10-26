import { useContext } from "react";
import { AppContext } from "../../contexts/AppContext";
import { useUpdate } from "../../contexts/UpdateContext";
import {
  CheckIcon,
  DownloadIcon,
  ExternalLinkIcon,
  RefreshIcon,
  SpinnerIcon,
  WarningIcon,
} from "../../functions/icon";
import { BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import CloseButton from "../CloseButton";

interface UpdateStatusProps {
  onDismiss?: () => void;
}

export default function UpdateStatus({ onDismiss }: UpdateStatusProps = {}) {
  const appContext = useContext(AppContext);
  const { data: { info, progress }, actions: updateActions } = useUpdate();

  const {
    system: { os },
    build: { version, dev },
    isLoading
  } = appContext.data;

  const manualDownloadUrl = "https://github.com/djip-odoo/obox-app/releases/latest";
  const isUpdateAvailable = !!info.latestVersion && info.latestVersion !== info.currentVersion;

  if (isLoading) {
    return (
      <div className="animate-pulse bg-white/90 border border-gray-200/80 rounded-2xl p-4 text-xs flex items-center gap-3.5 shadow-xs backdrop-blur-xs">
        <div className="w-9 h-9 rounded-xl bg-gray-100 shrink-0" />
        <div className="space-y-1.5 flex-1 min-w-0">
          <div className="h-3.5 bg-gray-200 rounded-md w-32" />
          <div className="h-2.5 bg-gray-100 rounded-md w-48" />
        </div>
      </div>
    );
  }

  return (
    <>
      {/* STATE: checking */}
      {info.state === "checking" && (
        <div
          key="checking"
          className="animate-fade-in bg-linear-to-r from-gray-50/95 via-purple-50/20 to-gray-50/95 border border-gray-200/90 rounded-2xl p-4 text-xs flex items-center gap-3.5 shadow-xs backdrop-blur-xs"
        >
          <div className="relative w-9 h-9 rounded-xl bg-odoo/10 text-odoo flex items-center justify-center shrink-0 border border-odoo/20 shadow-2xs">
            <SpinnerIcon className="w-4 h-4 animate-spin text-odoo" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="font-semibold text-gray-900 leading-tight">Checking for updates…</p>
            <p className="text-[11px] text-gray-500 mt-0.5">Connecting to GitHub releases server</p>
          </div>
        </div>
      )}

      {/* STATE: downloading */}
      {info.state === "downloading" && (
        <div
          key="downloading"
          className="animate-fade-in bg-linear-to-b from-white to-purple-50/30 border border-odoo/20 rounded-2xl p-4 text-xs space-y-3 shadow-sm backdrop-blur-xs"
        >
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2.5 min-w-0">
              <div className="w-7 h-7 rounded-lg bg-odoo/10 text-odoo flex items-center justify-center shrink-0 border border-odoo/20">
                <div className="w-3.5 h-3.5 border-2 border-odoo border-t-transparent rounded-full animate-spin" />
              </div>
              <span className="font-semibold text-gray-900 truncate">Downloading update…</span>
            </div>
            <span className="font-mono text-xs font-bold text-odoo bg-odoo/10 px-2.5 py-0.5 rounded-full border border-odoo/20 shadow-2xs">
              {progress}%
            </span>
          </div>

          <div className="h-2 rounded-full bg-gray-100 overflow-hidden p-0.5 border border-gray-200/70">
            <div
              className="h-full rounded-full bg-linear-to-r from-odoo via-odoo-light to-amber-500 transition-all duration-300 ease-out shadow-xs"
              style={{ width: `${Math.max(progress, 3)}%` }}
            />
          </div>

          <div className="flex items-center justify-between text-[11px] pt-0.5">
            <span className="text-gray-500">Please wait while the download completes.</span>
            <button
              type="button"
              onClick={updateActions.handleCancel}
              className="text-gray-500 hover:text-red-600 font-medium hover:underline cursor-pointer transition-colors px-1.5 py-0.5 rounded"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* STATE: installing */}
      {info.state === "installing" && (
        <div
          key="installing"
          className="animate-fade-in bg-linear-to-r from-purple-50/70 via-white to-purple-50/40 border border-odoo/25 rounded-2xl p-4 text-xs flex items-center gap-3.5 shadow-sm backdrop-blur-xs"
        >
          <div className="w-9 h-9 rounded-xl bg-odoo/10 text-odoo flex items-center justify-center shrink-0 border border-odoo/20 shadow-2xs">
            <div className="w-4 h-4 border-2 border-odoo border-t-transparent rounded-full animate-spin" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="font-semibold text-gray-900 leading-tight">Installing update…</p>
            <p className="text-[11px] text-gray-600 mt-0.5 leading-relaxed">
              {os === "windows"
                ? "The application will close to run the installer. Please leave 'Run Obox App' checked to restart."
                : "The new binary is being installed. You will be prompted to restart once finished."}
            </p>
          </div>
        </div>
      )}

      {/* STATE: installed (Binary replaced; restart required) */}
      {info.state === "installed" && (
        <div
          key="installed"
          className="animate-fade-in bg-linear-to-br from-purple-50/90 via-white to-emerald-50/40 border border-odoo/30 rounded-2xl p-4 text-xs space-y-2.5 shadow-sm backdrop-blur-xs"
        >
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-start gap-3 min-w-0">
              <div className="w-9 h-9 rounded-xl bg-odoo text-white flex items-center justify-center shrink-0 mt-0.5 shadow-xs ring-4 ring-odoo/10">
                <CheckIcon className="w-4 h-4" strokeWidth={2.5} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <p className="font-semibold text-gray-900 leading-tight">
                    Update installed
                  </p>
                  <span className="font-mono text-[10px] font-bold text-odoo bg-odoo/10 px-1.5 py-0.5 rounded border border-odoo/20">
                    Restart required
                  </span>
                </div>
                <p className="text-[11px] text-gray-600 mt-1 leading-relaxed">
                  Version <span className="font-mono font-semibold text-gray-900">{info.latestVersion || "new"}</span> has been installed. Please restart Obox App to apply changes.
                </p>
              </div>
            </div>
            {onDismiss && <CloseButton onClick={onDismiss} />}
          </div>
        </div>
      )}

      {/* STATE: idle (Up to date) */}
      {info.state === "idle" && (
        <div
          key="idle"
          className="animate-fade-in bg-linear-to-r from-emerald-50/90 via-emerald-50/50 to-teal-50/30 border border-emerald-200/90 rounded-2xl p-4 text-xs flex items-center justify-between gap-3 text-emerald-950 shadow-2xs backdrop-blur-xs"
        >
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-9 h-9 rounded-xl bg-emerald-500/10 text-emerald-600 flex items-center justify-center shrink-0 border border-emerald-500/20 shadow-2xs">
              <CheckIcon className="w-4 h-4" strokeWidth={2.5} />
            </div>
            <div className="min-w-0">
              {dev ? (
                <>
                  <p className="font-semibold text-emerald-950 leading-tight">Dev build — updates skipped</p>
                  <p className="text-[11px] text-emerald-700 mt-0.5">
                    Update checks are disabled in development mode.
                  </p>
                </>
              ) : (
                <>
                  <p className="font-semibold text-emerald-950 leading-tight">Already up to date</p>
                  <p className="text-[11px] text-emerald-700 mt-0.5">
                    You are running the latest version of Obox App.
                  </p>
                </>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={updateActions.checkForUpdate}
            className="group inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-emerald-800 bg-white hover:bg-emerald-100/70 border border-emerald-200/90 rounded-xl shadow-2xs active:scale-95 transition-all duration-150 cursor-pointer shrink-0"
            title="Check again"
          >
            <RefreshIcon className="w-3.5 h-3.5 text-emerald-700 group-hover:rotate-180 transition-transform duration-300" />
            Check Again
          </button>
        </div>
      )}

      {/* STATE: update_available (Newer version available) */}
      {info.state === "update_available" && info && (
        <div
          key="update_available"
          className="animate-fade-in bg-linear-to-br from-white via-purple-50/15 to-white border border-odoo/20 rounded-2xl p-4 text-xs space-y-3 shadow-xs backdrop-blur-xs"
        >
          {/* Top Row: Icon + Version Info + Close Button */}
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-start gap-3 min-w-0 flex-1">
              <div className="w-9 h-9 rounded-xl bg-odoo/10 text-odoo flex items-center justify-center shrink-0 mt-0.5 border border-odoo/20 shadow-2xs">
                <DownloadIcon className="w-4 h-4" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-semibold text-gray-900 leading-tight">Update Available</span>
                  <span className="font-mono text-[11px] font-bold text-odoo bg-odoo/10 px-2 py-0.5 rounded-md border border-odoo/20">
                    {info.latestVersion}
                  </span>
                </div>
                <p className="text-[11px] text-gray-500 mt-0.5 leading-relaxed truncate">
                  {info.currentVersion
                    ? `Upgrade You are running version ${info.currentVersion}`
                    : `Version ${info.latestVersion} is ready to install`}
                </p>
              </div>
            </div>
            {onDismiss && <CloseButton onClick={onDismiss} />}
          </div>

          {/* Optional Release Notes (compact & subtle) */}
          {info.notes && (
            <div className="bg-gray-50/90 rounded-xl p-2.5 border border-gray-200/70 text-[11px] text-gray-600 leading-relaxed max-h-20 overflow-y-auto whitespace-pre-line shadow-2xs">
              {info.notes}
            </div>
          )}

          {/* Action Buttons Row */}
          <div className="flex items-center gap-2 pt-0.5">
            {os === "darwin" ? (
              <button
                type="button"
                onClick={() => BrowserOpenURL(manualDownloadUrl)}
                className="flex-1 inline-flex items-center justify-center gap-2 px-3.5 py-2 text-xs font-semibold text-white bg-odoo hover:bg-odoo-dark active:scale-[0.98] rounded-xl shadow-xs transition-all duration-150 cursor-pointer"
              >
                <DownloadIcon className="w-3.5 h-3.5" />
                <span>Download from GitHub</span>
              </button>
            ) : (
              <button
                type="button"
                onClick={updateActions.handleUpdate}
                className="flex-1 inline-flex items-center justify-center gap-2 px-3.5 py-2 text-xs font-semibold text-white bg-odoo hover:bg-odoo-dark active:scale-[0.98] rounded-xl shadow-xs transition-all duration-150 cursor-pointer"
              >
                <DownloadIcon className="w-3.5 h-3.5" />
                <span>Download & Install</span>
              </button>
            )}

            {onDismiss ? (
              <button
                type="button"
                onClick={onDismiss}
                className="px-3.5 py-2 text-xs font-medium text-gray-600 hover:text-gray-900 bg-gray-50 hover:bg-gray-100 border border-gray-200/90 rounded-xl transition-colors cursor-pointer"
              >
                Later
              </button>
            ) : os !== "darwin" ? (
              <button
                type="button"
                onClick={() => BrowserOpenURL(manualDownloadUrl)}
                title="Download from GitHub"
                className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-medium text-gray-600 hover:text-gray-900 bg-gray-50 hover:bg-gray-100 border border-gray-200/90 rounded-xl transition-colors cursor-pointer shrink-0"
              >
                <span>Manual</span>
                <ExternalLinkIcon className="w-3 h-3 text-gray-400" />
              </button>
            ) : null}
          </div>
        </div>
      )}

      {/* STATE: failed (Error during check, download, or install) */}
      {info.state === "failed" && (
        <div
          key="failed"
          className="animate-fade-in bg-linear-to-br from-red-50/95 via-rose-50/60 to-red-50/40 border border-red-200/90 rounded-2xl p-4 text-xs space-y-3 shadow-2xs backdrop-blur-xs text-red-950"
        >
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-start gap-3 min-w-0 flex-1">
              <div className="w-9 h-9 rounded-xl bg-red-500/10 text-red-600 flex items-center justify-center shrink-0 mt-0.5 border border-red-500/20 shadow-2xs">
                <WarningIcon className="w-4 h-4" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="font-semibold text-red-950 leading-tight">
                  {isUpdateAvailable ? "Update failed" : "Unable to check for updates"}
                </p>
                <div className="mt-1.5 p-2 bg-white/70 rounded-lg border border-red-200/60 shadow-2xs">
                  <p className="text-[11px] text-red-800 leading-relaxed font-mono-none">
                    {info.error || "Cannot reach update server. Please check your internet connection."}
                  </p>
                </div>
              </div>
            </div>

            <div className="flex items-center gap-1.5 shrink-0">
              {(!(isUpdateAvailable) || os !== "darwin") && (
                <button
                  type="button"
                  onClick={isUpdateAvailable ? updateActions.handleUpdate : updateActions.checkForUpdate}
                  className="group inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-red-800 bg-white hover:bg-red-100/70 border border-red-200/90 rounded-xl shadow-2xs active:scale-95 transition-all duration-150 cursor-pointer shrink-0"
                >
                  <RefreshIcon className="w-3.5 h-3.5 text-red-700 group-hover:rotate-180 transition-transform duration-300" />
                  {isUpdateAvailable ? "Retry Update" : "Retry"}
                </button>
              )}
              {onDismiss && <CloseButton onClick={onDismiss} />}
            </div>
          </div>

          {/* Manual download link on error */}
          <div className="pt-2 border-t border-red-200/60 flex items-center justify-between gap-2 text-[11px]">
            <span className="text-red-700">Prefer manual installation?</span>
            <div className="flex items-center gap-2.5">
              <button
                type="button"
                onClick={() => BrowserOpenURL(manualDownloadUrl)}
                className="text-odoo font-medium underline inline-flex items-center gap-1 hover:text-odoo-dark cursor-pointer transition-colors"
              >
                <span>Download Manually</span>
                <ExternalLinkIcon className="w-3 h-3" />
              </button>
              {onDismiss && (
                <button
                  type="button"
                  onClick={onDismiss}
                  className="text-gray-500 hover:text-gray-800 cursor-pointer"
                >
                  Dismiss
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
