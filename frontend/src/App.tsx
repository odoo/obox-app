import { useEffect, useState } from "react";
import NetworkPrinting from "./components/NetworkPrinting";
import NetworkPrintingEnabledDialog from "./components/NetworkPrintingEnabledDialog";
import PrinterList from "./components/PrinterList";
import HeaderBar from "./components/HeaderBar";
import SetPinDialog from "./components/SetPinDialog";
import PINModal from "./components/PINModal";
import { backendService } from "./services/backend";
import AppProviders from "./contexts/AppProviders";

export default function App() {
  return (
    <AppProviders>
      <AppContent />
    </AppProviders>
  );
}

function AppContent() {
  const [pendingPin, setPendingPin] = useState(false);

  useEffect(() => {
    if (typeof window !== "undefined") {
      backendService.setWailsAppURL(
        window.location.origin + window.location.pathname
      );
    }

    const isExit =
      typeof window !== "undefined" &&
      (window.location.search.includes("kiosk_exit=1") ||
        window.location.hash.includes("kiosk_exit=1"));

    if (isExit) {
      try {
        window.history.replaceState({}, document.title, window.location.pathname);
      } catch (err) { }
      backendService.returnToWailsApp();
      setPendingPin(true);
      return;
    }

    backendService.isPendingPinAuth().then((pending) => {
      if (pending) {
        setPendingPin(true);
      }
    });
  }, []);

  const handlePinSuccess = async () => {
    setPendingPin(false);
    await backendService.completePinAuth(true);
  };

  const handlePinDismiss = async () => {
    setPendingPin(false);
    await backendService.completePinAuth(false);
  };

  return (
    <>
      <div className="h-screen w-screen overflow-hidden flex flex-col font-sans bg-gray-50">
        <HeaderBar />
        <main className="flex-1 min-h-0 flex flex-col items-center justify-center p-4 sm:p-6 overflow-hidden">
          <section className="w-full sm:max-w-md md:max-w-lg lg:max-w-xl flex flex-col min-h-0">
            <PrinterList />
          </section>
          <NetworkPrintingEnabledDialog />
          <NetworkPrinting />
          <SetPinDialog />
        </main>
      </div>
      {pendingPin && (
        <PINModal
          onSuccess={handlePinSuccess}
          onDismiss={handlePinDismiss}
          title="Security PIN"
          subtitle="Enter PIN to access Wails management"
        />
      )}
    </>
  );
}
