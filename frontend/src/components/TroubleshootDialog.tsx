import { useState } from "react";
import Dialog from "./Dialog";
import { useClipboard } from "../hooks/useClipboard";
import { CheckIcon, CopyIcon, HelpCircleIcon, SettingsIcon } from "../functions/icon";

interface CodeSnippetProps {
  label: string;
  command: string;
  description?: string;
}

function CodeSnippet({ label, command, description }: CodeSnippetProps) {
  const { copy, isCopied } = useClipboard();

  return (
    <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-semibold text-gray-800">{label}</span>
        <button
          type="button"
          onClick={() => copy(command, label, label)}
          className="
            inline-flex items-center gap-1 rounded-md border border-gray-300
            bg-white px-2 py-1 text-[11px] font-medium text-gray-700
            shadow-2xs transition-colors hover:border-gray-400 hover:bg-gray-50
            cursor-pointer
          "
        >
          {isCopied(label) ? (
            <>
              <CheckIcon className="h-3 w-3 text-green-600" strokeWidth={2.5} />
              <span className="text-green-600 font-medium">Copied</span>
            </>
          ) : (
            <>
              <CopyIcon className="h-3 w-3 text-gray-500" />
              <span>Copy</span>
            </>
          )}
        </button>
      </div>

      {description && (
        <p className="mt-1 text-[11px] text-gray-500">{description}</p>
      )}

      <div className="mt-2 overflow-x-auto rounded-lg bg-gray-900 px-3 py-2 text-[11px] font-mono text-green-400">
        <code>{command}</code>
      </div>
    </div>
  );
}

export default function TroubleshootDialog() {
  const [activeTab, setActiveTab] = useState<"service" | "edge" | "faq">("service");

  const fullServiceScript = `:: Create the Windows Service
sc.exe create odoo-kiosk binPath= "\\"C:\\Program Files\\ePOS proxy\\ePOS proxy\\ePOS proxy.exe\\" --kiosk" start= auto obj= "LocalSystem"

:: Start the service
sc.exe start odoo-kiosk

:: Query status (should report STATE: 4 RUNNING)
sc.exe query odoo-kiosk`;

  const { copy: copyAll, isCopied: isCopiedAll } = useClipboard();

  return (
    <Dialog
      title="Troubleshoot & Setup Guide"
      actions={[
        {
          name: "close",
          label: "Close",
          variant: "secondary",
        },
      ]}
      openButton={
        <button
          type="button"
          className="
            w-full flex items-center justify-between
            rounded-xl border border-gray-200 bg-white hover:bg-gray-50
            px-4 py-3 transition-colors cursor-pointer shadow-2xs
          "
        >
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-blue-50 text-blue-600">
              <SettingsIcon className="h-5 w-5" />
            </div>
            <div className="text-left">
              <div className="text-sm font-semibold text-gray-800">
                Troubleshoot & Kiosk Setup
              </div>
              <div className="text-xs text-gray-500">
                Windows Service commands and Edge Kiosk guide
              </div>
            </div>
          </div>
          <span className="rounded-full bg-blue-50 border border-blue-200 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wide text-blue-600">
            Guide
          </span>
        </button>
      }
    >
      <div className="flex flex-col gap-5 text-gray-700">
        {/* Navigation Tabs */}
        <div className="flex rounded-lg bg-gray-100 p-1 text-xs font-medium">
          <button
            type="button"
            onClick={() => setActiveTab("service")}
            className={`
              flex-1 rounded-md py-1.5 transition-colors cursor-pointer text-center
              ${activeTab === "service"
                ? "bg-white text-gray-900 shadow-2xs font-semibold"
                : "text-gray-600 hover:text-gray-900"
              }
            `}
          >
            Windows Service
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("edge")}
            className={`
              flex-1 rounded-md py-1.5 transition-colors cursor-pointer text-center
              ${activeTab === "edge"
                ? "bg-white text-gray-900 shadow-2xs font-semibold"
                : "text-gray-600 hover:text-gray-900"
              }
            `}
          >
            Edge Kiosk Guide
          </button>
        </div>

        {/* TAB 1: WINDOWS SERVICE */}
        {activeTab === "service" && (
          <div className="flex flex-col gap-4">
            <div className="rounded-xl border border-amber-200 bg-amber-50/70 p-3 text-xs text-amber-900">
              <div className="flex items-center gap-1.5 font-semibold">
                <HelpCircleIcon className="h-4 w-4 shrink-0 text-amber-600" />
                Administrator Prompt Required
              </div>
              <p className="mt-1 text-[11px] leading-relaxed text-amber-800">
                Open <strong>Command Prompt</strong> or <strong>PowerShell</strong> as <strong>Administrator</strong> to register and start the service.
              </p>
            </div>

            <div className="flex items-center justify-between">
              <span className="text-xs font-bold uppercase tracking-wider text-gray-500">
                Quick Setup Commands
              </span>
              <button
                type="button"
                onClick={() => copyAll(fullServiceScript, "fullServiceScript", "All service setup commands")}
                className="
                  inline-flex items-center gap-1 rounded-lg bg-odoo px-2.5 py-1
                  text-[11px] font-medium text-white shadow-2xs transition-colors
                  hover:bg-odoo-dark cursor-pointer
                "
              >
                {isCopiedAll("fullServiceScript") ? "Copied All!" : "Copy All Commands"}
              </button>
            </div>

            <div className="flex flex-col gap-3">
              <CodeSnippet
                label="1. Create Windows Service"
                description="Registers ePOS proxy as an auto-start service under LocalSystem."
                command='sc.exe create odoo-kiosk binPath= "\"C:\Program Files\ePOS proxy\ePOS proxy\ePOS proxy.exe\" --kiosk" start= auto obj= "LocalSystem"'
              />

              <CodeSnippet
                label="2. Start Service"
                description="Starts the background server immediately without rebooting."
                command="sc.exe start odoo-kiosk"
              />

              <CodeSnippet
                label="3. Query Status"
                description="Verifies the service state reports STATE : 4 RUNNING."
                command="sc.exe query odoo-kiosk"
              />
            </div>

            <div className="mt-1 border-t border-gray-200 pt-3">
              <span className="text-xs font-bold uppercase tracking-wider text-gray-500">
                Service Management
              </span>
              <div className="mt-2.5 grid grid-cols-1 sm:grid-cols-2 gap-2">
                <div className="rounded-lg border border-gray-200 bg-gray-50 p-2.5">
                  <div className="text-[11px] font-semibold text-gray-700">Stop Service</div>
                  <div className="mt-1 font-mono text-[10px] text-gray-800">sc.exe stop odoo-kiosk</div>
                </div>
                <div className="rounded-lg border border-gray-200 bg-gray-50 p-2.5">
                  <div className="text-[11px] font-semibold text-gray-700">Delete / Uninstall</div>
                  <div className="mt-1 font-mono text-[10px] text-gray-800">sc.exe delete odoo-kiosk</div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* TAB 2: EDGE KIOSK GUIDE */}
        {activeTab === "edge" && (
          <div className="flex flex-col gap-4 text-xs">
            {/* Assigned Access */}
            <div className="rounded-xl border border-gray-200 bg-white p-3.5 shadow-2xs">
              <div className="flex items-center gap-2 font-semibold text-gray-900">
                <span className="flex h-5 w-5 items-center justify-center rounded-full bg-odoo/10 text-odoo text-[11px] font-bold">
                  A
                </span>
                Method: Windows Assigned Access (Recommended)
              </div>
              <p className="mt-1 text-[11px] text-gray-500">
                Locks down Windows into a dedicated single-app kiosk with automatic login.
              </p>

              <ol className="mt-3 flex flex-col gap-2 list-decimal list-inside text-[11px] text-gray-700">
                <li>
                  Open Windows <strong>Settings</strong> &gt; <strong>Accounts</strong> &gt; <strong>Other users</strong> (or <em>Family & other users</em>).
                </li>
                <li>
                  Under <strong>Set up a kiosk</strong>, select <strong>Assigned access</strong> &gt; <strong>Get started</strong>.
                </li>
                <li>
                  Enter a name for the kiosk account (e.g. <code className="bg-gray-100 px-1 py-0.5 rounded font-mono">KioskUser</code>).
                </li>
                <li>
                  Choose <strong>Microsoft Edge</strong> as the kiosk application.
                </li>
                <li>
                  Select <strong>As a digital sign or interactive display</strong> (full screen, no address bar or tabs).
                </li>
                <li>
                  Enter the POS URL: <code className="bg-gray-100 px-1 py-0.5 rounded font-mono">http://127.0.0.1:4545/</code>
                </li>
                <li>
                  Restart the machine. Windows will sign in to the kiosk account and launch Edge automatically.
                </li>
              </ol>
            </div>
          </div>
        )}

        {/* TAB 3: DIAGNOSTICS & FAQ */}
        {activeTab === "faq" && (
          <div className="flex flex-col gap-3 text-xs">
            <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-3">
              <div className="font-semibold text-gray-900">
                Why was there an "EBWebView access denied" error previously?
              </div>
              <p className="mt-1 text-[11px] leading-relaxed text-gray-600">
                When launched under the <code className="font-mono bg-white px-1 py-0.5 rounded">LocalSystem</code> account, services run in Session 0 without an interactive desktop. If Wails attempted to launch Edge WebView2, it failed because the system profile has no writable user data folder. With native Windows Service support, ePOS proxy starts in headless server mode and bypasses WebView2 completely.
              </p>
            </div>

            <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-3">
              <div className="font-semibold text-gray-900">
                What address and port does the proxy listen on?
              </div>
              <p className="mt-1 text-[11px] leading-relaxed text-gray-600">
                The service binds to <code className="font-mono bg-white px-1 py-0.5 rounded">0.0.0.0:4545</code> (or up to 4555 if 4545 is occupied). You can verify it is active by opening <a href="http://127.0.0.1:4545/" target="_blank" rel="noreferrer" className="text-odoo font-medium underline">http://127.0.0.1:4545/</a> in any browser.
              </p>
            </div>

            <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-3">
              <div className="font-semibold text-gray-900">
                How do I view service logs?
              </div>
              <p className="mt-1 text-[11px] leading-relaxed text-gray-600">
                When running as a service, logs are stored in <code className="font-mono bg-white px-1 py-0.5 rounded">%ProgramData%\EposProxy\logs\obox-app.log</code> (typically <code className="font-mono bg-white px-1 py-0.5 rounded">C:\ProgramData\EposProxy\logs</code>).
              </p>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
}
