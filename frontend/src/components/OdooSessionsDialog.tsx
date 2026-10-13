import { useContext, useEffect, useState } from "react";
import { AppContext } from "../contexts/AppContext";
import { ToastContext } from "../contexts/ToastContext";
import Dialog from "./Dialog";
import { backendService } from "../services/backend";
import { config } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { errorText } from "../error";
import {
  CheckCircleSolidIcon,
  CheckSolidIcon,
  CloseIcon,
  ExternalLinkIcon,
  GlobeAltIcon,
  LinkIcon,
  RefreshIcon,
  ServerStackIcon,
  SpinnerIcon,
  WarningIcon,
} from "../functions/icon";

export interface OdooSessionsDialogProps {
  openSignal?: number;
  onClose?: () => void;
}

export default function OdooSessionsDialog({
  openSignal: externalOpenSignal,
  onClose,
}: OdooSessionsDialogProps) {
  const { data: { isWails } } = useContext(AppContext);
  const toastContext = useContext(ToastContext);

  const [dialogOpenSignal, setDialogOpenSignal] = useState(0);
  const [instances, setInstances] = useState<config.OdooInstance[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  // Form states
  const [newUrl, setNewUrl] = useState("");
  const [newName, setNewName] = useState("");
  const [isAdding, setIsAdding] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  // Action states
  const [openingId, setOpeningId] = useState<string | null>(null);
  const [activatingId, setActivatingId] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);
  const [syncingId, setSyncingId] = useState<string | null>(null);

  const loadInstances = async () => {
    try {
      setIsLoading(true);
      const list = await backendService.getOdooInstances();
      setInstances(list || []);
    } catch (err) {
      console.error("Failed to load Odoo instances:", err);
    } finally {
      setIsLoading(false);
    }
  };

  // Sync external open signal
  useEffect(() => {
    if (externalOpenSignal && externalOpenSignal > 0) {
      setDialogOpenSignal((prev) => prev + 1);
      loadInstances();
    }
  }, [externalOpenSignal]);

  // Listen to Wails native events
  useEffect(() => {
    if (!isWails) return;

    const unlistenMenu = EventsOn("open-odoo-sessions-dialog", () => {
      setDialogOpenSignal((prev) => prev + 1);
      loadInstances();
    });

    const unlistenChanged = EventsOn(
      "odoo-instances-changed",
      (updatedList: config.OdooInstance[]) => {
        if (Array.isArray(updatedList)) {
          setInstances(updatedList);
        } else {
          loadInstances();
        }
      }
    );

    return () => {
      if (unlistenMenu) unlistenMenu();
      if (unlistenChanged) unlistenChanged();
    };
  }, [isWails]);

  const handleAddInstance = async (e: React.FormEvent) => {
    e.preventDefault();
    const trimmedUrl = newUrl.trim();
    if (!trimmedUrl) {
      setFormError("Odoo instance URL is required.");
      return;
    }

    try {
      setIsAdding(true);
      setFormError(null);
      const added = await backendService.addOdooInstance(trimmedUrl, newName.trim());
      setNewUrl("");
      setNewName("");
      toastContext.actions.showToast(
        `Added Odoo instance: ${added.name || added.url}`,
        "success"
      );
      await loadInstances();
    } catch (err) {
      setFormError(errorText(err, "Failed to add Odoo instance."));
    } finally {
      setIsAdding(false);
    }
  };

  const handleOpenSession = async (inst: config.OdooInstance) => {
    try {
      setOpeningId(inst.id);
      await backendService.openOdooSession(inst.id);
      toastContext.actions.showToast(`Navigating to ${inst.name || inst.url}...`, "success");
      onClose?.();
    } catch (err) {
      toastContext.actions.showToast(
        `Failed to open Odoo session: ${errorText(err, "unknown error")}`,
        "danger"
      );
    } finally {
      setOpeningId(null);
    }
  };

  const handleSetActive = async (inst: config.OdooInstance) => {
    try {
      setActivatingId(inst.id);
      await backendService.setActiveOdooInstance(inst.id);
      toastContext.actions.showToast(
        `Switched active connection to ${inst.name || inst.url}`,
        "success"
      );
      await loadInstances();
    } catch (err) {
      toastContext.actions.showToast(
        `Failed to set active instance: ${errorText(err, "unknown error")}`,
        "danger"
      );
    } finally {
      setActivatingId(null);
    }
  };

  const handleSync = async (inst: config.OdooInstance) => {
    try {
      setSyncingId(inst.id);
      await backendService.syncOdooInstance(inst.id);
      toastContext.actions.showToast(
        `Successfully synced device state with ${inst.name || inst.url}`,
        "success"
      );
      await loadInstances();
    } catch (err) {
      toastContext.actions.showToast(
        `Failed to sync with Odoo: ${errorText(err, "unknown error")}`,
        "danger"
      );
    } finally {
      setSyncingId(null);
    }
  };

  const handleRemove = async (inst: config.OdooInstance) => {
    if (
      !window.confirm(
        `Are you sure you want to remove ${inst.name || inst.url}? This will disconnect any paired session.`
      )
    ) {
      return;
    }

    try {
      setRemovingId(inst.id);
      await backendService.removeOdooInstance(inst.id);
      toastContext.actions.showToast(
        `Removed Odoo instance ${inst.name || inst.url}`,
        "success"
      );
      await loadInstances();
    } catch (err) {
      toastContext.actions.showToast(
        `Failed to remove instance: ${errorText(err, "unknown error")}`,
        "danger"
      );
    } finally {
      setRemovingId(null);
    }
  };

  return (
    <Dialog
      title="Odoo Sessions & Device Pairing"
      size="xl"
      openSignal={dialogOpenSignal}
      onClose={onClose}
      onOpen={loadInstances}
      actions={[
        {
          name: "close",
          label: "Close",
          variant: "secondary",
        },
      ]}
    >
      <div className="space-y-5 text-gray-800">
        {/* Information Banner */}
        <div className="flex items-start gap-3 rounded-2xl border border-purple-100 bg-purple-50/70 p-3.5 text-xs text-purple-950">
          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-odoo text-white shadow-xs">
            <GlobeAltIcon className="h-4 w-4" />
          </div>
          <div className="space-y-1">
            <p className="font-semibold text-odoo">
              Connect to Odoo & Pair OBOX Device
            </p>
            <p className="text-[11px] leading-relaxed text-gray-600">
              Save your Odoo server instances below. Click <strong>Open & Log in</strong> to open Odoo directly inside the app. Once logged in, click the floating <strong>Pair with this Odoo</strong> badge to securely link your device using your authenticated session.
            </p>
          </div>
        </div>

        {/* Section: Saved Instances */}
        <div className="rounded-2xl border border-gray-200/90 bg-white p-4 shadow-2xs">
          <div className="flex items-center justify-between pb-3 border-b border-gray-100 mb-3">
            <div className="flex items-center gap-2">
              <ServerStackIcon className="h-4 w-4 text-gray-500" />
              <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700">
                Configured Instances ({instances.length})
              </h4>
            </div>
            {isLoading && (
              <span className="flex items-center gap-1.5 text-[11px] text-gray-400">
                <SpinnerIcon className="h-3 w-3 animate-spin" />
                Refreshing...
              </span>
            )}
          </div>

          {instances.length === 0 ? (
            <div className="py-8 text-center text-gray-400">
              <GlobeAltIcon className="mx-auto h-8 w-8 text-gray-300 stroke-[1.5]" />
              <p className="mt-2 text-xs font-medium text-gray-600">
                No Odoo instances configured yet
              </p>
              <p className="mt-0.5 text-[11px] text-gray-400">
                Add your Odoo database or server URL using the form below.
              </p>
            </div>
          ) : (
            <div className="space-y-2.5">
              {instances.map((inst) => {
                const isPaired = Boolean(inst.token);
                const isActive = inst.is_active;

                return (
                  <div
                    key={inst.id}
                    className={`flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-3.5 rounded-xl border transition-all ${
                      isActive
                        ? "border-emerald-200 bg-emerald-50/30 shadow-xs"
                        : "border-gray-100 bg-gray-50/50 hover:bg-white hover:border-gray-200"
                    }`}
                  >
                    <div className="min-w-0 flex-1 space-y-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-xs font-semibold text-gray-900 truncate">
                          {inst.name || new URL(inst.url).hostname}
                        </span>

                        {isActive && (
                          <span className="inline-flex items-center gap-1 rounded-md bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-800">
                            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
                            Active Connection
                          </span>
                        )}

                        {isPaired ? (
                          <span className="inline-flex items-center gap-1 rounded-md bg-purple-100 px-2 py-0.5 text-[10px] font-medium text-odoo">
                            <CheckSolidIcon className="h-2.5 w-2.5" />
                            Paired {inst.obox_id ? `(#${inst.obox_id})` : ""}
                          </span>
                        ) : (
                          <span className="inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-500">
                            Not Paired
                          </span>
                        )}
                      </div>

                      <div className="flex items-center gap-1 text-[11px] font-mono text-gray-500 truncate">
                        <LinkIcon className="h-3 w-3 shrink-0 text-gray-400" />
                        <span className="truncate">{inst.url}</span>
                      </div>
                    </div>

                    {/* Action buttons */}
                    <div className="flex items-center gap-2 shrink-0 self-end sm:self-center">
                      {isPaired && !isActive && (
                        <button
                          type="button"
                          disabled={activatingId === inst.id}
                          onClick={() => handleSetActive(inst)}
                          className="px-2.5 py-1.5 text-xs font-medium rounded-lg border border-gray-200 bg-white text-gray-700 hover:bg-gray-50 hover:border-gray-300 transition-colors cursor-pointer disabled:opacity-50"
                        >
                          {activatingId === inst.id ? (
                            <SpinnerIcon className="h-3 w-3 inline animate-spin mr-1" />
                          ) : null}
                          Set Active
                        </button>
                      )}

                      {isPaired && (
                        <button
                          type="button"
                          disabled={syncingId === inst.id}
                          onClick={() => handleSync(inst)}
                          className="flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium rounded-lg border border-purple-200 bg-purple-50 text-purple-900 hover:bg-purple-100 hover:border-purple-300 transition-colors cursor-pointer disabled:opacity-50"
                          title="Sync device state and printers with this Odoo instance"
                        >
                          {syncingId === inst.id ? (
                            <SpinnerIcon className="h-3 w-3 animate-spin" />
                          ) : (
                            <RefreshIcon className="h-3 w-3 text-odoo" />
                          )}
                          <span>Sync</span>
                        </button>
                      )}

                      <button
                        type="button"
                        disabled={openingId === inst.id}
                        onClick={() => handleOpenSession(inst)}
                        className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg bg-odoo text-white hover:bg-odoo/90 shadow-2xs transition-all active:scale-98 cursor-pointer disabled:opacity-50"
                        title="Open in desktop WebView to log in and pair"
                      >
                        {openingId === inst.id ? (
                          <SpinnerIcon className="h-3.5 w-3.5 animate-spin" />
                        ) : (
                          <ExternalLinkIcon className="h-3.5 w-3.5" />
                        )}
                        <span>Open & Log in</span>
                      </button>

                      <button
                        type="button"
                        disabled={removingId === inst.id}
                        onClick={() => handleRemove(inst)}
                        className="p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors cursor-pointer disabled:opacity-50"
                        title="Remove instance"
                      >
                        <CloseIcon className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Section: Add New Instance Form */}
        <form
          onSubmit={handleAddInstance}
          className="rounded-2xl border border-gray-200/90 bg-white p-4 shadow-2xs space-y-3"
        >
          <div className="flex items-center gap-2 pb-2 border-b border-gray-100">
            <GlobeAltIcon className="h-4 w-4 text-odoo" />
            <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700">
              Add Odoo Instance
            </h4>
          </div>

          {formError && (
            <div className="flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 p-2.5 text-xs text-red-700">
              <WarningIcon className="h-4 w-4 shrink-0 mt-0.5 text-red-500" />
              <span>{formError}</span>
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            <div className="sm:col-span-2 space-y-1">
              <label
                htmlFor="odoo-url-input"
                className="block text-[11px] font-semibold text-gray-600"
              >
                Odoo Database / Server URL <span className="text-red-500">*</span>
              </label>
              <div className="relative">
                <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
                  <LinkIcon className="h-3.5 w-3.5" />
                </div>
                <input
                  id="odoo-url-input"
                  type="text"
                  placeholder="https://mycompany.odoo.com or http://localhost:8069"
                  value={newUrl}
                  onChange={(e) => {
                    setNewUrl(e.target.value);
                    if (formError) setFormError(null);
                  }}
                  className="w-full rounded-xl border border-gray-200 bg-gray-50/50 py-2 pl-9 pr-3 text-xs font-mono outline-none transition-all placeholder:text-gray-400 placeholder:font-sans focus:border-odoo focus:bg-white focus:ring-2 focus:ring-odoo/10"
                />
              </div>
            </div>

            <div className="space-y-1">
              <label
                htmlFor="odoo-name-input"
                className="block text-[11px] font-semibold text-gray-600"
              >
                Display Name <span className="text-gray-400 font-normal">(optional)</span>
              </label>
              <input
                id="odoo-name-input"
                type="text"
                placeholder="e.g. Main POS Server"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                className="w-full rounded-xl border border-gray-200 bg-gray-50/50 py-2 px-3 text-xs outline-none transition-all placeholder:text-gray-400 focus:border-odoo focus:bg-white focus:ring-2 focus:ring-odoo/10"
              />
            </div>
          </div>

          <div className="flex justify-end pt-1">
            <button
              type="submit"
              disabled={isAdding || !newUrl.trim()}
              className="flex items-center gap-1.5 px-4 py-2 text-xs font-semibold rounded-xl bg-gray-900 text-white hover:bg-black transition-all cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed shadow-xs"
            >
              {isAdding ? (
                <>
                  <SpinnerIcon className="h-3.5 w-3.5 animate-spin" />
                  <span>Adding...</span>
                </>
              ) : (
                <span>Add Instance</span>
              )}
            </button>
          </div>
        </form>
      </div>
    </Dialog>
  );
}
