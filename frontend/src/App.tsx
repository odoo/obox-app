import NetworkPrinting from "./components/NetworkPrinting";
import NetworkPrintingEnabledDialog from "./components/NetworkPrintingEnabledDialog";
import PrinterList from "./components/PrinterList";
import { AppContextWrapper } from "./contexts/AppContext";
import { PrinterContextWrapper } from "./contexts/PrinterContext";
import { ToastContextWrapper } from "./contexts/ToastContext";
import { OdooContextWrapper } from "./contexts/OdooContext";

function App() {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <PrinterContextWrapper>
          <OdooContextWrapper>
            <div className="min-h-screen flex flex-col items-center justify-center p-4 sm:p-6 font-sans bg-gray-50">
              <PrinterList />
              <NetworkPrintingEnabledDialog />
              <NetworkPrinting />
            </div>
          </OdooContextWrapper>
        </PrinterContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}

export default App;
