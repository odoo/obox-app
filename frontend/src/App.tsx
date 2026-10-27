import HeaderBar from "./components/HeaderBar";
import NetworkPrinting from "./components/NetworkPrinting";
import NetworkPrintingEnabledDialog from "./components/NetworkPrintingEnabledDialog";
import PrinterList from "./components/PrinterList";
import AppProviders from "./contexts/AppProviders";

export default function App() {
  return (
    <AppProviders>
      <div className="h-screen w-screen overflow-hidden flex flex-col font-sans bg-gray-50">
        <HeaderBar />
        <main className="flex-1 min-h-0 flex flex-col items-center justify-center p-4 sm:p-6 overflow-hidden">
          <section className="w-full sm:max-w-md md:max-w-lg lg:max-w-xl flex flex-col min-h-0">
            <PrinterList />
          </section>
          <NetworkPrintingEnabledDialog />
          <NetworkPrinting />
        </main>
      </div>
    </AppProviders>
  );
}
