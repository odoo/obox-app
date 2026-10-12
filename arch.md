# ePOS Proxy — System Architecture & Technical Specification

> **Version**: 2.15.0  
> **Target Audience**: AI Agents & Engineers  
> **Core Stack**: Go 1.25+ / 1.26+, Wails v2 (v2.15.0), React 18, TypeScript, Tailwind CSS, Fiber v3, libusb (`gousb`).

---

## 1. Executive Summary & Purpose

**ePOS Proxy** is a cross-platform desktop application and embedded local HTTP proxy server designed to connect **Odoo Point of Sale (POS)** web clients to physical ESC/POS receipt and label printers.

### Primary Capabilities
1. **Hardware Print Proxy**: Bridges web-based POS print requests (XML receipt formats, raw ESC/POS, images) to USB and LAN thermal printers.
2. **Dual Execution Runtime**:
   - **Desktop Application (Wails POS Station)**: Runs native windowed UI or fullscreen Kiosk mode on the physical POS terminal.
   - **Remote Webview (Admin Panel)**: Serves the exact same React management interface over LAN at `http://<lan-ip>:<port>/` for remote administration from phones, tablets, or other workstations.
3. **Hardware Kiosk Mode**: Fullscreen POS kiosk execution with a secret 4-tap corner unlock gesture, configurable URL validation patterns, and remote open/close/reload triggers.
4. **Security & Privilege Model**: Read-only monitoring over LAN; privileged operations (printing, drawer triggers, LAN printer modification, kiosk configuration) are gated behind a 4-digit security PIN with session-based tokens.

---

## 2. System Architecture Diagram

```
+-----------------------------------------------------------------------------------+
|                              ePOS Proxy System                                    |
|                                                                                   |
|  +---------------------------+               +---------------------------------+  |
|  |     Wails Desktop UI      |               |      Remote Web Browser         |  |
|  |  (React SPA / Wails ctx)  |               |  (Phone/Tablet/PC via LAN /)    |  |
|  +-------------+-------------+               +----------------+----------------+  |
|                |                                              |                   |
|       Wails IPC Bindings                              HTTP REST + Bearer Token    |
|                |                                              |                   |
|                v                                              v                   |
|  +---------------------------+               +---------------------------------+  |
|  |      app.go (App)         |               |   internal/server/ (Fiber v3)   |  |
|  |  - Fullscreen / Menu      | <===========> |   - Catch-all Static SPA (/)    |  |
|  |  - Desktop Event Emitter  |  Server Events|   - /api/printers, /api/webview |  |
|  |  - Native OS Dialogs      |   & Callbacks |   - /api/auth/session (PIN)     |  |
|  +-------------+-------------+               +----------------+----------------+  |
|                |                                              |                   |
|                +----------------------+-----------------------+                   |
|                                       |                                           |
|                                       v                                           |
|                      +----------------------------------+                         |
|                      |         Core Subsystems          |                         |
|                      |  - internal/config/ (Manager)    |                         |
|                      |  - internal/printer/ (Manager)   |                         |
|                      |  - internal/escpos/ (Raster/Cmd) |                         |
|                      |  - internal/logger/ (Logrus/Zip) |                         |
|                      +----------------+-----------------+                         |
|                                       |                                           |
+---------------------------------------+-------------------------------------------+
                                        |
                 +----------------------+----------------------+
                 |                                             |
                 v                                             v
     +-----------------------+                     +-----------------------+
     |   USB ESC/POS Printer |                     |   LAN Network Printer |
     | (gousb / libusb-1.0)  |                     |   (Raw TCP Port 9100) |
     +-----------------------+                     +-----------------------+
```

---

## 3. Directory & File Organization

```
epos-proxy/
├── main.go                     # Application entry point, single-instance lock, menu init, Wails setup
├── app.go                      # App struct: Wails JS bindings, lifecycle hooks, fullscreen/kiosk control
├── app_test.go                 # App unit tests and mock integration tests
├── menu.go                     # Native OS application menu configuration ("Set PIN", "Logs", etc.)
├── go.mod / go.sum             # Go module definition (Wails v2.15.0, Fiber v3, gousb)
├── wails.json                  # Wails build and project configuration
│
├── internal/
│   ├── config/                 # Persistent configuration manager (config.json)
│   │   ├── config.go           # AppConfig schema, port range resolution (4545-4555), PIN, URL validation
│   │   └── config_test.go      # Configuration serialization, PIN validation, URL format tests
│   │
│   ├── escpos/                 # ESC/POS command builders, font rasterizers, drawer pulses
│   ├── logger/                 # Rolling file logging (lumberjack.v2) and zip log downloader
│   │
│   ├── printer/                # Printer discovery, hardware management, test print generation
│   │   ├── printer.go          # Manager struct, USB discovery, printer status caching
│   │   ├── usb.go              # gousb / libusb device claims and endpoint writing
│   │   └── lan.go              # LAN TCP printer connections, network status checkers
│   │
│   └── server/                 # Embedded Fiber v3 HTTP server
│       ├── server.go           # Fiber routing, static SPA embed serving (/), CORS, auth middleware
│       ├── api_handlers.go     # REST API handler implementations for remote clients
│       └── server_test.go      # HTTP server integration and authenticated API tests
│
└── frontend/                   # React 18 + TypeScript + Tailwind CSS SPA
    ├── index.html              # HTML shell with mobile viewport settings
    ├── vite.config.ts          # Vite build config with Wails asset pipeline
    ├── src/
    │   ├── App.tsx             # Root component with context wrappers and main responsive layout
    │   ├── main.tsx            # React DOM root entry
    │   │
    │   ├── services/           # Unified Runtime Backend Driver / Middleware
    │   │   └── backend.ts      # IBackendService adapter resolving methods per runtime (Wails vs Remote)
    │   │
    │   ├── api/                # API client layer for Remote Webview
    │   │   ├── authState.ts    # Token storage singleton (Wails token vs sessionStorage Bearer token)
    │   │   └── client.ts       # Type-safe fetch wrappers mirroring Wails Go bindings
    │   │
    │   ├── contexts/           # React Context Providers
    │   │   ├── AppContext.tsx      # Unified Application & Runtime context (isWails, ready, serverURL, OS, serverRunning)
    │   │   ├── PrinterContext.tsx  # Printer list state, polling, LAN status tracking
    │   │   ├── WebViewContext.tsx  # Kiosk configuration, fullscreen state, reload triggers
    │   │   ├── PINContext.tsx      # Global modal state for PIN entry/auth
    │   │   └── ToastContext.tsx    # Notification toasts (success / danger)
    │   │
    │   ├── hooks/              # Custom React Hooks
    │   │   ├── useClipboard.ts     # Reusable clipboard copy with timer reset and toasts
    │   │   ├── usePINGate.ts       # Wraps privileged actions with PIN verification on remote browsers
    │   │   ├── useMountTransition.ts # Animation mount/unmount transitions
    │   │   └── useStepDialog.ts    # Multi-step wizard dialog logic
    │   │
    │   └── components/         # UI Components
    │       ├── Dialog.tsx          # Responsive modal / mobile bottom-sheet with fixed bottom actions
    │       ├── PrinterList.tsx     # Card container for connected/available printers
    │       ├── PrinterListItem.tsx # Individual printer row with status indicators & delete button
    │       ├── PrinterActions.tsx  # Action buttons: Copy IP, Test Print, Open Cash Drawer
    │       ├── WebViewDialog.tsx   # Redesigned Kiosk & Remote Access configuration dialog
    │       ├── RemoteKioskSection.tsx # QR code and LAN URL display for remote connection
    │       ├── KioskOverlay.tsx    # Fullscreen iframe overlay with secret 4-tap corner unlock gesture
    │       ├── PINModal.tsx        # Responsive on-screen numpad for PIN entry, confirm, and unlock
    │       └── SetPinDialog.tsx    # Native menu listener to configure 4-digit PIN in desktop app
```

---

## 4. Subsystem Deep-Dives

### 4.1. Network & Web Server (`internal/server`)

The application starts an embedded **Fiber v3** server on a dynamically resolved port between `4545` and `4555`.

#### Static Asset Serving
- The compiled React application (`frontend/dist`) is embedded into the Go binary via `embed.FS`.
- The server serves `frontend/dist` directly on `/` using a catch-all route `s.app.Get("/*", ...)`. Any client (desktop or remote browser) accessing `http://<ip>:<port>/` loads the management SPA.

#### API Endpoints Overview
| Method | Endpoint | Auth Required | Description |
|---|---|:---:|---|
| `GET` | `/` | No | Serves embedded React single-page application |
| `POST` | `/cgi-bin/epos/service.cgi` | No | Odoo ePOS XML print service endpoint |
| `POST` | `/hw_proxy/print_xml_receipt` | No | Odoo hardware proxy print endpoint |
| `GET` | `/api/app` | No | App variables (port, autostart, network printing) |
| `GET` | `/api/printers` | No | List of USB and LAN printers |
| `GET` | `/api/printers/lan/:ip/status` | No | Check reachability of a LAN printer IP |
| `GET` | `/api/webview` | No | Kiosk configuration (`url`, `enabled`, `hasPIN`) |
| `GET` | `/api/troubleshoot` | No | System troubleshooting info (LAN IPs, ports) |
| `POST` | `/api/auth/session` | No | Validate 4-digit PIN and receive Bearer session token |
| `POST` | `/api/printers/lan` | **Yes** | Add a new LAN printer IP |
| `DELETE`| `/api/printers/lan` | **Yes** | Remove a LAN printer IP |
| `POST` | `/api/printers/:printerId/test-print` | **Yes** | Trigger hardware test print |
| `POST` | `/api/printers/:printerId/cash-drawer` | **Yes** | Send cash drawer kick pulse |
| `POST` | `/api/webview/url` | **Yes** | Update Kiosk target URL |
| `POST` | `/api/webview/enabled` | **Yes** | Enable or disable Desktop Kiosk mode remotely |
| `POST` | `/api/webview/reload` | **Yes** | Broadcast reload event to active desktop kiosk screen |

#### Authentication Mechanism (`RequireAuth`)
- **Desktop Wails App**: On startup, Go generates a cryptographically secure UUID `sessionToken`. Wails sends this token via header `X-Wails-Token`. It is trusted unconditionally without prompting the user.
- **Remote Webview**: Requests authenticate via `Authorization: Bearer <session-token>` obtained from `POST /api/auth/session`.
- Privileged endpoints reject requests missing valid credentials with `HTTP 401 Unauthorized`.

---

### 4.2. Kiosk Mode & Remote Control Flow

Kiosk mode displays an embedded POS terminal fullscreen.

```
[ Remote Web Browser ]                         [ Go Server ]                      [ Desktop Wails App ]
         |                                           |                                      |
         | 1. User enters PIN & clicks "Open Kiosk"  |                                      |
         |------------------------------------------>|                                      |
         |    POST /api/webview/enabled {true}       |                                      |
         |    (Bearer Token)                         |                                      |
         |                                           | 2. SetWebViewEnabled(true)           |
         |                                           |    Invoke onKioskChanged(true)       |
         |                                           |------------------------------------->|
         |                                           |    wailsruntime.EventsEmit(          |
         |                                           |      "kiosk-state-changed", true     |
         |                                           |    )                                 |
         |                                           |                                      | 3. isKioskActive = true
         |                                           |                                      |    SetWindowFullscreen(true)
         |                                           |                                      |    Hides native menu bar
         |                                           |                                      |    Mounts KioskOverlay iframe
```

#### Key Rules of Kiosk Execution:
1. **Desktop-Only View**: Fullscreen kiosk mode and the `<KioskOverlay />` iframe mount **strictly on the desktop Wails process** (`isWails === true`). The remote webview operates solely as an administrative console.
2. **Secret Unlock Gesture**: While fullscreen on desktop, tapping the **top-right corner 4 times quickly** within 1000ms triggers the floating PIN unlock modal.
3. **Instant Remote Triggering**: Remote browsers can open, close, or reload the desktop kiosk screen via authenticated API calls.
4. **URL Constraint Validation**: URLs are validated against `KIOSK_URL_CONSTRAINTS` (supporting Odoo self-ordering formats like `https://domain.com/pos-self/204?access_token=...`). Empty or invalid URLs disable kiosk launch buttons automatically.

---

### 4.3. PIN Security & Management

- **Storage**: Stored in `config.json` (`webview_pin`). Default initial PIN is `"0000"`.
- **Desktop-Only Configuration**: To prevent unauthorized tampering over LAN, the PIN can **only be created or changed directly inside the desktop application** via the native App Menu (**App Menu -> Set PIN**).
- **Remote Immediate PIN Overlay & Gate ([PINContext.tsx](file:///home/odoo/odoo/epos-proxy/frontend/src/contexts/PINContext.tsx) & [usePINGate.ts](file:///home/odoo/odoo/epos-proxy/frontend/src/hooks/usePINGate.ts))**:
  - Upon initial load or refresh in remote browser context, if unauthenticated, the application immediately presents the `<PINModal />` numpad overlay.
  - Entering the 4-digit PIN issues an in-memory session token that **strictly clears on every page refresh**.
  - **Persistent Cooldown & Attempts**: Failed PIN attempts and 30s lockout cooldowns persist in `localStorage` (`epos-pin-attempts`, `epos-pin-cooldown-until`) so refreshing the page cannot bypass rate-limiting or lockout timers.
  - All subsequent actions within the authenticated page session (Test Print, Cash Drawer, Add/Delete LAN Printer, Open/Close Kiosk, Save URL, Reload Kiosk) execute seamlessly on the **1st click**.

---

### 4.4. Hardware Printing Subsystem (`internal/printer` & `internal/escpos`)

1. **USB Printers**:
   - Discovered using `google/gousb` iterating USB descriptors for Printer class (`0x07`).
   - Claimed exclusively during active print jobs and released immediately after data transfer.
2. **LAN Printers**:
   - Stored by IPv4 string in `config.json`.
   - Communicates via raw TCP socket (standard port `9100`).
   - Status checks verify socket connectivity with a 1.5s timeout.
3. **Cash Drawer Execution**:
   - Transmits ESC/POS pulse command `ESC p m t1 t2` (`\x1B\x70\x00\x19\xFA`) to the designated printer.

---

## 5. Frontend State Architecture

```
                       +-------------------------+
                       |   RuntimeContext.tsx    |  --> Determines isWails vs Remote Browser
                       +------------+------------+
                                    |
          +-------------------------+-------------------------+
          |                         |                         |
+---------v---------+     +---------v---------+     +---------v---------+
|   AppContext      |     |  PrinterContext   |     |  WebViewContext   |
| - Port, Autostart |     | - Printers list   |     | - Kiosk URL       |
| - NetworkPrinting |     | - LAN status map  |     | - isKioskActive   |
| - TroubleshootInfo|     | - Add/Remove LAN  |     | - reloadNonce     |
+-------------------+     +-------------------+     +-------------------+
```

### Context Isolation & Responsiveness:
- **`RuntimeContext`**: Initialized at startup by inspecting `window.go`. If `window.go` is present, `isWails` is `true`. Otherwise, all calls route through `api/client.ts`.
- **`useClipboard`**: Custom hook managing clipboard copying, active feedback timers, and notification toasts.
- **`Dialog.tsx`**: Mobile-responsive sheet/modal architecture with fixed title header, scrollable content body (`flex-1 overflow-y-auto`), and sticky action footer (`sticky bottom-0`).

---

## 6. Build, Verification & Testing Commands

### Go Backend Tests
```bash
# Run all unit and integration tests fresh without cache
go test -count=1 ./...

# Run server package tests with verbose logging
go test -v ./internal/server/...

# Run configuration tests
go test -v ./internal/config/...
```

### Frontend Build & Type Check
```bash
# Navigate to frontend directory
cd frontend

# Run TypeScript typecheck & Vite production bundle build
npm run build
```

### Running Development Environment
```bash
# Run Wails live development mode (launches desktop window + Vite HMR server)
wails dev
```

---

## 7. Common Gotchas & Architectural Rules for AI Agents

1. **Never Mount Kiosk in Remote Browsers**: `KioskOverlay.tsx` must always check `if (!isWails) return null;`.
2. **Never Call `enterKiosk()` Inside `toggleEnabled()`**: Circular calls between `toggleEnabled` and `exitKiosk/enterKiosk` will trigger infinite call stack recursion. Keep actions decoupled.
3. **Pure-Go Window & Menu State**: Control fullscreen and menubar dynamically via `wailsruntime.WindowFullscreen`, `wailsruntime.WindowUnfullscreen`, and `wailsruntime.MenuSetApplicationMenu` without native CGO overrides.
4. **PIN Updates Stay Local**: Do not expose a remote HTTP endpoint for changing the PIN. PIN setting belongs exclusively to `menu.go` and `SetPinDialog.tsx`.
5. **Always Use `useClipboard`**: For any clipboard copy operations in the UI, use the unified `useClipboard` hook for consistent feedback and toast notifications.
