import AboutDialog from "./components/AboutDialog";
import NetworkPrinting from "./components/NetworkPrinting";
import NetworkPrintingEnabledDialog from "./components/NetworkPrintingEnabledDialog";
import PrinterList from "./components/PrinterList";
import UpdateBanner from "./components/UpdateBanner";
import { AppContextWrapper } from "./contexts/AppContext";
import { PrinterContextWrapper } from "./contexts/PrinterContext";
import { ToastContextWrapper } from "./contexts/ToastContext";
import { UpdateContextWrapper } from "./contexts/UpdateContext";

function App() {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <PrinterContextWrapper>
          <UpdateContextWrapper>
            <div className="min-h-screen flex flex-col items-center justify-center p-4 sm:p-6 font-sans bg-gray-50 relative">
              <AboutDialog />
              <div className="w-full flex flex-col items-center justify-center">
                <PrinterList />
                <NetworkPrintingEnabledDialog />
                <NetworkPrinting />
                <UpdateBanner />
              </div>
            </div>
          </UpdateContextWrapper>
        </PrinterContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}

export default App;
