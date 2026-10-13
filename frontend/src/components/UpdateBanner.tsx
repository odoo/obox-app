import { useContext } from "react";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { UpdateContext } from "../contexts/UpdateContext";
import CloseButton from "./CloseButton";

export default function UpdateBanner() {
  const updateContext = useContext(UpdateContext);
  if (!updateContext) return null;

  const {
    info,
    downloading,
    applying,
    progress,
    failed,
    errorMessage,
    bannerDismissed,
  } = updateContext.data;
  const { handleUpdate, handleCancel, handleDismiss } = updateContext.actions;

  const showBanner = !bannerDismissed && !!info && info.state === "update_available";

  return (
    <>
      {showBanner && !downloading && !applying && (
        <div className="animate-fade-in fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-semibold text-gray-800">
                Update available: {info.latestVersion}
              </p>
              <p className="text-xs text-gray-500 mt-0.5">
                You are running version {info.currentVersion}.
              </p>
            </div>
            <CloseButton onClick={handleDismiss} />
          </div>

          {info.notes && info.notes.length > 0 && (
            <p className="text-xs text-gray-600 mt-2 whitespace-pre-line line-clamp-3">
              {info.notes}
            </p>
          )}

          {failed ? (
            <div className="mt-3 p-3 bg-red-50 border border-red-200 rounded-lg text-xs space-y-2">
              <p className="font-semibold text-red-700">
                {errorMessage || info?.error || "Update failed. You can download it directly:"}
              </p>
              <div className="space-y-2">
                <button
                  type="button"
                  onClick={() => BrowserOpenURL("https://github.com/djip-odoo/obox-app/releases/latest")}
                  className="w-full px-3 py-1.5 rounded-md bg-odoo text-white text-xs font-medium hover:opacity-90 transition-opacity cursor-pointer"
                >
                  Download Directly from GitHub
                </button>
              </div>
              <div className="flex items-center gap-2 pt-1">
                <button
                  type="button"
                  onClick={handleUpdate}
                  className="flex-1 px-3 py-1 text-center text-xs font-medium text-odoo bg-white border border-odoo/30 rounded-md hover:bg-odoo/5 transition-colors cursor-pointer"
                >
                  Retry Update
                </button>
                <button
                  type="button"
                  onClick={handleDismiss}
                  className="px-3 py-1 text-center text-xs font-medium text-gray-500 hover:text-gray-800 transition-colors cursor-pointer"
                >
                  Later
                </button>
              </div>
            </div>
          ) : (
            <div className="flex items-center gap-2 mt-3">
              <button
                type="button"
                onClick={handleUpdate}
                className="flex-1 px-4 py-2 rounded-lg bg-odoo text-white text-sm font-medium hover:opacity-90 transition-opacity cursor-pointer text-center"
              >
                Download & Install
              </button>
              <button
                type="button"
                onClick={handleDismiss}
                className="px-3 py-2 rounded-lg border border-gray-200 text-gray-600 text-sm font-medium hover:bg-gray-50 transition-colors cursor-pointer"
              >
                Later
              </button>
            </div>
          )}
        </div>
      )}

      {showBanner && downloading && (
        <div className="animate-fade-in fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
          <div className="flex items-center justify-between">
            <p className="text-sm font-semibold text-gray-800">
              Downloading update {info.latestVersion}…
            </p>
            <button
              type="button"
              onClick={handleCancel}
              className="text-xs text-gray-500 hover:text-gray-800 font-medium px-2 py-0.5 rounded hover:bg-gray-100 transition-colors cursor-pointer"
            >
              Cancel
            </button>
          </div>
          <div className="mt-3">
            <div className="h-2 rounded-full bg-gray-200 overflow-hidden">
              <div
                className="h-full bg-odoo transition-all duration-200"
                style={{ width: `${Math.max(progress, 2)}%` }}
              />
            </div>
            <p className="text-xs text-gray-500 mt-1 font-mono">{progress}% downloaded</p>
          </div>
        </div>
      )}

      {showBanner && applying && info && (
        <div className="animate-fade-in fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
          <div className="flex items-center gap-3">
            <div className="w-5 h-5 border-2 border-odoo border-t-transparent rounded-full animate-spin shrink-0" />
            <div>
              <p className="text-sm font-semibold text-gray-800">
                Installing update {info.latestVersion}…
              </p>
              <p className="text-xs text-gray-500 mt-0.5">
                The app will close to complete the update.
              </p>
            </div>
          </div>
        </div>
      )}
    </>
  );
}