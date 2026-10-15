import { ReactNode } from "react";
import { ToastContextWrapper } from "./ToastContext";
import { AppContextWrapper } from "./AppContext";
import { PrinterContextWrapper } from "./PrinterContext";
import { OdooContextWrapper } from "./OdooContext";

export default function AppProviders({ children }: { children: ReactNode }) {
  return (
    <ToastContextWrapper>
      <AppContextWrapper>
        <PrinterContextWrapper>
          <OdooContextWrapper>{children}</OdooContextWrapper>
        </PrinterContextWrapper>
      </AppContextWrapper>
    </ToastContextWrapper>
  );
}
