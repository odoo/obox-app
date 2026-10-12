import { detectWails } from "../contexts/AppContext";
import {
  AddLANPrinter,
  AppVariable,
  CheckLANPrinterStatus,
  ConfirmRemoveLANPrinter,
  GetTroubleshootInfo,
  GetWebViewConfig,
  Printers,
  SetWebViewEnabled,
  SetWebViewExitCorners,
  SetWebViewFullscreen,
  SetWebViewPIN,
  SetWebViewURL,
  SetWindowFullscreen,
  ValidateWebViewPIN,
  IsPendingPinAuth,
  CompletePinAuth,
  NavigateToWebApp,
  ReturnToWailsApp,
  SetWailsAppURL,
  ToggleAutoStart,
  ToggleNetworkPrinting,
} from "../../wailsjs/go/main/App";
import {
  apiAddLANPrinter,
  apiCashDrawer,
  apiCreatePINSession,
  apiGetAppVariable,
  apiGetLANPrinterStatus,
  apiGetPrinters,
  apiGetTroubleshootInfo,
  apiGetWebViewConfig,
  apiRemoveLANPrinter,
  apiSetWebViewEnabled,
  apiSetWebViewExitCorners,
  apiSetWebViewFullscreen,
  apiSetWebViewURL,
  apiTestPrint,
  ApiAppVariable,
  ApiPrinter,
  ApiPrintersResponse,
  ApiTroubleshootInfo,
  ApiWebViewConfig,
  apiReloadKiosk,
  apiQuitApp,
} from "../api/client";
import { main, printer } from "../../wailsjs/go/models";

export interface IBackendService {
  readonly isWails: boolean;

  // App & System Info
  getAppVariable(): Promise<main.AppVariable | ApiAppVariable>;
  getTroubleshootInfo(): Promise<main.TroubleshootInfo | ApiTroubleshootInfo>;
  getNetworkPrintingEnabled(): Promise<boolean>;
  setNetworkPrintingEnabled(v: boolean): Promise<void>;
  getAutostart(): Promise<boolean>;
  setAutostart(v: boolean): Promise<void>;

  // Printers
  getPrinters(): Promise<printer.DiscoveryResult | ApiPrintersResponse>;
  checkLANPrinterStatus(ip: string): Promise<{ online: boolean }>;
  addLANPrinter(ip: string): Promise<void>;
  removeLANPrinter(ip: string): Promise<boolean>;

  // Kiosk & Webview
  getWebViewConfig(): Promise<main.WebViewConfig | ApiWebViewConfig>;
  setWebViewURL(url: string): Promise<void>;
  setWebViewEnabled(enabled: boolean): Promise<void>;
  setWebViewExitCorners(corners: string[]): Promise<void>;
  setWebViewFullscreen(fullscreen: boolean): Promise<void>;
  setWebViewPIN(pin: string): Promise<void>;
  validatePIN(pin: string): Promise<boolean>;
  setWindowFullscreen(fullscreen: boolean): Promise<void>;
  reloadKiosk(): Promise<void>;
  quitServer(): Promise<void>;
  isPendingPinAuth(): Promise<boolean>;
  completePinAuth(success: boolean): Promise<void>;
  navigateToWebApp(): Promise<void>;
  returnToWailsApp(): Promise<void>;
  setWailsAppURL(url: string): Promise<void>;
}

class WailsBackendService implements IBackendService {
  readonly isWails = true;

  getAppVariable(): Promise<main.AppVariable> {
    return AppVariable();
  }

  getTroubleshootInfo(): Promise<main.TroubleshootInfo> {
    return GetTroubleshootInfo();
  }

  async getNetworkPrintingEnabled(): Promise<boolean> {
    const vars = await AppVariable();
    return vars.networkPrintingEnabled;
  }

  async setNetworkPrintingEnabled(v: boolean): Promise<void> {
    const vars = await AppVariable();
    if (vars.networkPrintingEnabled !== v) {
      await ToggleNetworkPrinting();
    }
  }

  async getAutostart(): Promise<boolean> {
    const vars = await AppVariable();
    return vars.autoStart;
  }

  async setAutostart(v: boolean): Promise<void> {
    const vars = await AppVariable();
    if (vars.autoStart !== v) {
      await ToggleAutoStart();
    }
  }

  getPrinters(): Promise<printer.DiscoveryResult> {
    return Printers();
  }

  async checkLANPrinterStatus(ip: string): Promise<{ online: boolean }> {
    const online = await CheckLANPrinterStatus(ip);
    return { online: Boolean(online) };
  }

  addLANPrinter(ip: string): Promise<void> {
    return AddLANPrinter(ip);
  }

  async removeLANPrinter(ip: string): Promise<boolean> {
    const confirmed = await ConfirmRemoveLANPrinter(ip);
    return Boolean(confirmed);
  }


  getWebViewConfig(): Promise<main.WebViewConfig> {
    return GetWebViewConfig();
  }

  setWebViewURL(url: string): Promise<void> {
    return SetWebViewURL(url);
  }

  setWebViewEnabled(enabled: boolean): Promise<void> {
    return SetWebViewEnabled(enabled);
  }

  setWebViewExitCorners(corners: string[]): Promise<void> {
    return SetWebViewExitCorners(corners);
  }

  setWebViewFullscreen(fullscreen: boolean): Promise<void> {
    return SetWebViewFullscreen(fullscreen);
  }

  setWebViewPIN(pin: string): Promise<void> {
    return SetWebViewPIN(pin);
  }

  validatePIN(pin: string): Promise<boolean> {
    return ValidateWebViewPIN(pin);
  }

  setWindowFullscreen(fullscreen: boolean): Promise<void> {
    return SetWindowFullscreen(fullscreen);
  }

  reloadKiosk(): Promise<void> {
    const wailsApp = (window as unknown as { go?: { main?: { App?: { ReloadKiosk?: () => Promise<void> } } } })?.go?.main?.App;
    if (wailsApp?.ReloadKiosk) {
      return wailsApp.ReloadKiosk();
    }
    return Promise.resolve();
  }

  quitServer(): Promise<void> {
    const wailsApp = (window as unknown as { go?: { main?: { App?: { Quit?: () => Promise<void> } } } })?.go?.main?.App;
    if (wailsApp?.Quit) {
      return wailsApp.Quit();
    }
    return Promise.resolve();
  }

  isPendingPinAuth(): Promise<boolean> {
    return IsPendingPinAuth();
  }

  completePinAuth(success: boolean): Promise<void> {
    return CompletePinAuth(success);
  }

  navigateToWebApp(): Promise<void> {
    return NavigateToWebApp();
  }

  returnToWailsApp(): Promise<void> {
    return ReturnToWailsApp();
  }

  setWailsAppURL(url: string): Promise<void> {
    return SetWailsAppURL(url);
  }
}

class RemoteBackendService implements IBackendService {
  readonly isWails = false;

  getAppVariable(): Promise<ApiAppVariable> {
    return apiGetAppVariable();
  }

  getTroubleshootInfo(): Promise<ApiTroubleshootInfo> {
    return apiGetTroubleshootInfo();
  }

  getNetworkPrintingEnabled(): Promise<boolean> {
    return Promise.resolve(false);
  }

  setNetworkPrintingEnabled(): Promise<void> {
    return Promise.resolve();
  }

  getAutostart(): Promise<boolean> {
    return Promise.resolve(false);
  }

  setAutostart(): Promise<void> {
    return Promise.resolve();
  }

  getPrinters(): Promise<ApiPrintersResponse> {
    return apiGetPrinters();
  }

  checkLANPrinterStatus(ip: string): Promise<{ online: boolean }> {
    return apiGetLANPrinterStatus(ip);
  }

  async addLANPrinter(ip: string): Promise<void> {
    await apiAddLANPrinter(ip);
  }

  async removeLANPrinter(ip: string): Promise<boolean> {
    await apiRemoveLANPrinter(ip);
    return true;
  }

  async testPrint(printer: printer.Device | ApiPrinter): Promise<void> {
    await apiTestPrint(printer.id);
  }

  async openCashDrawer(printer: printer.Device | ApiPrinter): Promise<void> {
    await apiCashDrawer(printer.id);
  }

  getWebViewConfig(): Promise<ApiWebViewConfig> {
    return apiGetWebViewConfig();
  }

  async setWebViewURL(url: string): Promise<void> {
    await apiSetWebViewURL(url);
  }

  async setWebViewEnabled(enabled: boolean): Promise<void> {
    await apiSetWebViewEnabled(enabled);
  }

  async setWebViewExitCorners(corners: string[]): Promise<void> {
    await apiSetWebViewExitCorners(corners);
  }

  async setWebViewFullscreen(fullscreen: boolean): Promise<void> {
    await apiSetWebViewFullscreen(fullscreen);
  }

  setWebViewPIN(): Promise<void> {
    return Promise.reject(new Error("PIN configuration is only permitted on the desktop application"));
  }

  validatePIN(pin: string): Promise<boolean> {
    return apiCreatePINSession(pin);
  }

  setWindowFullscreen(): Promise<void> {
    return Promise.resolve();
  }

  async reloadKiosk(): Promise<void> {
    await apiReloadKiosk();
  }

  async quitServer(): Promise<void> {
    await apiQuitApp();
  }

  isPendingPinAuth(): Promise<boolean> {
    return Promise.resolve(false);
  }

  completePinAuth(): Promise<void> {
    return Promise.resolve();
  }

  navigateToWebApp(): Promise<void> {
    return Promise.resolve();
  }

  returnToWailsApp(): Promise<void> {
    return Promise.resolve();
  }

  setWailsAppURL(): Promise<void> {
    return Promise.resolve();
  }
}

class DynamicBackendService implements IBackendService {
  private wails = new WailsBackendService();
  private remote = new RemoteBackendService();

  private get service(): IBackendService {
    return detectWails() ? this.wails : this.remote;
  }

  get isWails(): boolean {
    return detectWails();
  }

  getAppVariable(): Promise<main.AppVariable | ApiAppVariable> {
    return this.service.getAppVariable();
  }

  getTroubleshootInfo(): Promise<main.TroubleshootInfo | ApiTroubleshootInfo> {
    return this.service.getTroubleshootInfo();
  }

  getNetworkPrintingEnabled(): Promise<boolean> {
    return this.service.getNetworkPrintingEnabled();
  }

  setNetworkPrintingEnabled(v: boolean): Promise<void> {
    return this.service.setNetworkPrintingEnabled(v);
  }

  getAutostart(): Promise<boolean> {
    return this.service.getAutostart();
  }

  setAutostart(v: boolean): Promise<void> {
    return this.service.setAutostart(v);
  }

  getPrinters(): Promise<printer.DiscoveryResult | ApiPrintersResponse> {
    return this.service.getPrinters();
  }

  checkLANPrinterStatus(ip: string): Promise<{ online: boolean }> {
    return this.service.checkLANPrinterStatus(ip);
  }

  addLANPrinter(ip: string): Promise<void> {
    return this.service.addLANPrinter(ip);
  }

  removeLANPrinter(ip: string): Promise<boolean> {
    return this.service.removeLANPrinter(ip);
  }

  getWebViewConfig(): Promise<main.WebViewConfig | ApiWebViewConfig> {
    return this.service.getWebViewConfig();
  }

  setWebViewURL(url: string): Promise<void> {
    return this.service.setWebViewURL(url);
  }

  setWebViewEnabled(enabled: boolean): Promise<void> {
    return this.service.setWebViewEnabled(enabled);
  }

  setWebViewExitCorners(corners: string[]): Promise<void> {
    return this.service.setWebViewExitCorners(corners);
  }

  setWebViewFullscreen(fullscreen: boolean): Promise<void> {
    return this.service.setWebViewFullscreen(fullscreen);
  }

  setWebViewPIN(pin: string): Promise<void> {
    return this.service.setWebViewPIN(pin);
  }

  validatePIN(pin: string): Promise<boolean> {
    return this.service.validatePIN(pin);
  }

  setWindowFullscreen(fullscreen: boolean): Promise<void> {
    return this.service.setWindowFullscreen(fullscreen);
  }

  reloadKiosk(): Promise<void> {
    return this.service.reloadKiosk();
  }

  quitServer(): Promise<void> {
    return this.service.quitServer();
  }

  isPendingPinAuth(): Promise<boolean> {
    return this.service.isPendingPinAuth();
  }

  completePinAuth(success: boolean): Promise<void> {
    return this.service.completePinAuth(success);
  }

  navigateToWebApp(): Promise<void> {
    return this.service.navigateToWebApp();
  }

  returnToWailsApp(): Promise<void> {
    return this.service.returnToWailsApp();
  }

  setWailsAppURL(url: string): Promise<void> {
    return this.service.setWailsAppURL(url);
  }
}

export const backendService: IBackendService = new DynamicBackendService();
